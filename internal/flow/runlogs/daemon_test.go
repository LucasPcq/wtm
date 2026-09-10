package runlogs

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/testutil/socktest"
)

func pipedStream(t *testing.T) (*connStream, net.Conn) {
	t.Helper()
	client, daemon := net.Pipe()
	stream := newConnStream(connStreamParams{Conn: client, Name: "api", WorkDir: "/work/api"})
	t.Cleanup(func() {
		stream.Close()
		daemon.Close()
	})
	return stream, daemon
}

func TestConnStreamDeliversWhatTheJobWrote(t *testing.T) {
	stream, daemon := pipedStream(t)

	go daemon.Write([]byte("\x1b[32mready\x1b[0m"))

	select {
	case chunk := <-stream.Chunks():
		if string(chunk) != "\x1b[32mready\x1b[0m" {
			t.Fatalf("chunk = %q, want the bytes untouched", chunk)
		}
	case <-time.After(time.Second):
		t.Fatal("no chunk delivered")
	}
}

func TestConnStreamWritesToTheJobsStdin(t *testing.T) {
	stream, daemon := pipedStream(t)

	go stream.Write([]byte("q"))

	buf := make([]byte, 1)
	daemon.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := daemon.Read(buf); err != nil {
		t.Fatalf("read stdin: %v", err)
	}
	if buf[0] != 'q' {
		t.Fatalf("stdin = %q, want q", buf)
	}
}

func TestConnStreamClosesItsChunksWhenTheOutputEnds(t *testing.T) {
	stream, daemon := pipedStream(t)

	daemon.Close()

	select {
	case _, open := <-stream.Chunks():
		if open {
			t.Fatal("a chunk arrived after the job's output ended")
		}
	case <-time.After(time.Second):
		t.Fatal("the chunks channel stayed open")
	}
}

// Two things release the goroutine holding the connection, and a test that
// drains a near-empty queue proves neither: closing the connection wakes a read
// that is waiting for bytes, and the done channel wakes a delivery that no
// longer fits in the queue. One test each, or removing either leaves both green.

func TestConnStreamCloseWakesAReadWaitingForBytes(t *testing.T) {
	stream, _ := pipedStream(t)

	if err := stream.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}

	select {
	case _, open := <-stream.Chunks():
		if open {
			t.Fatal("a chunk arrived on a closed stream")
		}
	case <-time.After(time.Second):
		t.Fatal("the reader stayed parked on the connection after Close")
	}
}

func TestConnStreamCloseWakesADeliveryIntoAFullQueue(t *testing.T) {
	before := readersRunning()
	stream, daemon := pipedStream(t)

	go func() {
		for {
			if _, err := daemon.Write([]byte("line\n")); err != nil {
				return
			}
		}
	}()
	waitFor(t, "the queue to fill", func() bool {
		return len(stream.Chunks()) == domain.JobStreamQueueChunks
	})

	if err := stream.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Nothing reads the queue: the reader is parked on a delivery that does not
	// fit, and only the done channel can bring it back.
	waitFor(t, "the reader to leave", func() bool { return readersRunning() <= before })
}

func TestConnStreamRefusesWhatIsAskedOfAClosedStream(t *testing.T) {
	stream, daemon := pipedStream(t)

	if err := stream.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// A pane bubbletea already destroyed, running the resize command it had
	// scheduled: the daemon must never hear about it.
	if err := stream.Resize(Size{Cols: 80, Rows: 20}); !errors.Is(err, domain.ErrJobStreamClosed) {
		t.Fatalf("Resize after Close: %v, want ErrJobStreamClosed", err)
	}
	if err := stream.Write([]byte("q")); !errors.Is(err, domain.ErrJobStreamClosed) {
		t.Fatalf("Write after Close: %v, want ErrJobStreamClosed", err)
	}

	daemon.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
	if _, err := daemon.Read(make([]byte, 1)); err == nil {
		t.Fatal("the closed stream still reached the job")
	}
}

func TestConnStreamResizeWithoutADaemonClientIsRefused(t *testing.T) {
	stream, _ := pipedStream(t)

	if err := stream.Resize(Size{Cols: 80, Rows: 20}); err == nil {
		t.Fatal("Resize answered without a client to reach the daemon with")
	}
}

func readersRunning() int {
	buf := make([]byte, 1<<20)
	return strings.Count(string(buf[:runtime.Stack(buf, true)]), "(*connStream).read(")
}

func waitFor(t *testing.T, what string, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if done() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// scriptedDaemon answers the scripted request with the responses given, in
// order, then hangs up — enough to read what the adapter keeps of the daemon's
// answer. Listings are answered on their own, empty: a client checks the
// daemon's version with one before it sends anything that changes state.
func scriptedDaemon(t *testing.T, responses ...process.Response) string {
	t.Helper()
	socket := socktest.Path(t)
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { listener.Close() })

	held := make(chan struct{})
	t.Cleanup(func() { close(held) })

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go serveScript(conn, responses, held)
		}
	}()
	return socket
}

func serveScript(conn net.Conn, responses []process.Response, held <-chan struct{}) {
	defer conn.Close()

	var req process.Request
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		return
	}
	encoder := json.NewEncoder(conn)
	// Stamped like the real daemon's: a client refuses to talk to one whose
	// build it cannot match.
	if req.Action == process.ActionList {
		_ = encoder.Encode(process.Response{Status: process.StatusOK, Version: domain.Version})
		return
	}
	for _, resp := range responses {
		resp.Version = domain.Version
		if err := encoder.Encode(resp); err != nil {
			return
		}
	}
	// A daemon running a three-minute task holds the line: what the client does
	// about that is the client's business.
	<-held
}

func TestServiceStartKeepsWhatTheDaemonAnswered(t *testing.T) {
	failed := 1
	socket := scriptedDaemon(t,
		process.Response{Status: process.StatusOutput, Data: []byte("applying 001\n")},
		process.Response{Status: process.StatusError, Message: "task migrate failed: exit status 1", ExitCode: &failed},
	)

	var streamed []byte
	result, err := NewService(ServiceParams{SocketPath: socket}).Start(t.Context(), StartRequest{
		Job:      domain.JobConfig{Name: "migrate", Kind: domain.JobKindTask, Cmd: "pnpm migrate"},
		WorkDir:  "/work/api",
		OnOutput: func(chunk []byte) { streamed = append(streamed, chunk...) },
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	if !result.Refused || result.Message != "task migrate failed: exit status 1" {
		t.Fatalf("result = %+v, want the daemon's refusal", result)
	}
	if result.ExitCode == nil || *result.ExitCode != 1 {
		t.Fatalf("exit code = %v, want the 1 the daemon reported", result.ExitCode)
	}
	if got := string(streamed); got != "applying 001\n" {
		t.Fatalf("streamed %q, want the task's line", got)
	}
}

func TestServiceStartGivesUpOnTheDaemonWhenTheRunDetaches(t *testing.T) {
	socket := scriptedDaemon(t, process.Response{Status: process.StatusOutput, Data: []byte("applying 001\n")})
	ctx, detach := context.WithCancel(t.Context())
	service := NewService(ServiceParams{SocketPath: socket})

	streamed := make(chan struct{}, 1)
	answered := make(chan error, 1)
	go func() {
		_, err := service.Start(ctx, StartRequest{
			Job:     domain.JobConfig{Name: "migrate", Kind: domain.JobKindTask, Cmd: "pnpm migrate"},
			WorkDir: "/work/api",
			OnOutput: func([]byte) {
				select {
				case streamed <- struct{}{}:
				default:
				}
			},
		})
		answered <- err
	}()

	select {
	case <-streamed:
	case <-time.After(2 * time.Second):
		t.Fatal("the task never started streaming")
	}
	detach()

	select {
	case err := <-answered:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Start after the removal: %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		// Held on the socket read, with its goroutine, its connection and its
		// buffer, until the task the user walked away from ends.
		t.Fatal("Start held on to the daemon after the removal")
	}
}
