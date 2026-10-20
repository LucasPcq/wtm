package events

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/require"
)

// A fakeDaemon listens on a unix socket as wtm's daemon does, records each
// request line it reads and answers it; a test reads the requests with next.
type fakeDaemon struct {
	socket    string
	commonDir string
	requests  chan string
}

// listen starts a fakeDaemon that answers every request with success.
func listen(t *testing.T) *fakeDaemon {
	t.Helper()
	return listenAnswering(t, `{"status":"ok"}`)
}

// listenAnswering starts a fakeDaemon that answers every request with answer.
func listenAnswering(t *testing.T, answer string) *fakeDaemon {
	t.Helper()

	// A short directory under the system's temp: a unix socket path is limited to about 100 bytes.
	dir, err := os.MkdirTemp("", "wtm-ev")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	commonDir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	daemon := &fakeDaemon{socket: filepath.Join(dir, "wtm.sock"), commonDir: commonDir, requests: make(chan string, 16)}
	listener, err := net.Listen("unix", daemon.socket)
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			line, _ := bufio.NewReader(conn).ReadString('\n')
			daemon.requests <- line
			_, _ = conn.Write([]byte(answer + "\n"))
			_ = conn.Close()
		}
	}()

	return daemon
}

// next returns the next request the daemon received, failing the test after
// two seconds without one.
func (d *fakeDaemon) next(t *testing.T) string {
	t.Helper()
	select {
	case request := <-d.requests:
		return request
	case <-time.After(2 * time.Second):
		require.FailNow(t, "the daemon received nothing")
		return ""
	}
}

// publisherOn returns a Publisher of operation op-1 sending to daemon.
func publisherOn(t *testing.T, daemon *fakeDaemon) Publisher {
	t.Helper()
	return NewPublisher(PublisherParams{Root: "/repo", CommonDir: daemon.commonDir, SocketPath: daemon.socket, OpID: "op-1", CorrelationID: "popup-1", Now: fixedNow})
}

// payloadOf returns the event a request carries, as JSON.
func payloadOf(t *testing.T, request string) string {
	t.Helper()
	var envelope struct {
		Payload json.RawMessage `json:"payload"`
	}
	require.NoError(t, json.Unmarshal([]byte(request), &envelope))
	return string(envelope.Payload)
}

// withoutPayload returns request with its payload emptied, to check the
// envelope alone.
func withoutPayload(t *testing.T, request string) string {
	t.Helper()
	var envelope map[string]any
	require.NoError(t, json.Unmarshal([]byte(request), &envelope))
	envelope["payload"] = map[string]any{}
	data, err := json.Marshal(envelope)
	require.NoError(t, err)
	return string(data)
}

// decoded returns payload as a JSON object.
func decoded(t *testing.T, payload string) map[string]any {
	t.Helper()
	var event map[string]any
	require.NoError(t, json.Unmarshal([]byte(payload), &event))
	return event
}

func symlink(target, link string) error {
	return os.Symlink(target, link)
}

// socketDir returns a short new directory for a socket: a unix socket path is limited to about 100 bytes.
func socketDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "wtm-bus")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// serving starts a daemon on a new socket, idling out after idle, and returns its socket and the channel Serve's error arrives on.
func serving(t *testing.T, idle time.Duration) (string, <-chan error) {
	t.Helper()
	socket := filepath.Join(socketDir(t), "wtm.sock")
	ctx, cancel := context.WithCancel(context.Background())
	served := startServe(ctx, ServeParams{SocketPath: socket, Idle: idle})
	t.Cleanup(func() {
		cancel()
		<-served
	})
	require.Eventually(t, Bus{SocketPath: socket}.answers, 2*time.Second, 10*time.Millisecond)
	return socket, served
}

// startServe runs Serve with params in the background and returns the channel its error arrives on.
func startServe(ctx context.Context, params ServeParams) <-chan error {
	served := make(chan error, 1)
	go func() { served <- Serve(ctx, params) }()
	return served
}

// subscribed subscribes to repos on the bus at socket, ending with the test.
func subscribed(t *testing.T, socket string, repos ...string) <-chan []byte {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	lines, err := Bus{SocketPath: socket}.Subscribe(ctx, repos)
	require.NoError(t, err)
	return lines
}

// publish publishes payload under repo on the bus at socket, as a Publisher sends it.
func publish(t *testing.T, socket, repo, payload string) {
	t.Helper()
	conn, err := net.Dial("unix", socket)
	require.NoError(t, err)
	defer conn.Close()
	require.NoError(t, json.NewEncoder(conn).Encode(request{Action: actionPublish, Repo: repo, Payload: json.RawMessage(payload)}))
	var reply response
	require.NoError(t, json.NewDecoder(conn).Decode(&reply))
	require.Equal(t, statusOK, reply.Status)
}

// received returns the next line of lines, failing the test after two seconds without one or once lines is closed.
func received(t *testing.T, lines <-chan []byte) string {
	t.Helper()
	select {
	case line, open := <-lines:
		require.True(t, open, "the subscription ended")
		return string(line)
	case <-time.After(2 * time.Second):
		require.FailNow(t, "nothing received")
		return ""
	}
}

// closed reports whether lines is closed within two seconds, draining what it still holds.
func closed(lines <-chan []byte) bool {
	timeout := time.After(2 * time.Second)
	for {
		select {
		case _, open := <-lines:
			if !open {
				return true
			}
		case <-timeout:
			return false
		}
	}
}

// schemaV2 is events.v2.json compiled.
func schemaV2(t *testing.T) *jsonschema.Schema {
	t.Helper()
	file, err := os.Open("events.v2.json")
	require.NoError(t, err)
	defer file.Close()
	doc, err := jsonschema.UnmarshalJSON(file)
	require.NoError(t, err)
	compiler := jsonschema.NewCompiler()
	require.NoError(t, compiler.AddResource("events.v2.json", doc))
	schema, err := compiler.Compile("events.v2.json")
	require.NoError(t, err)
	return schema
}

// validates returns the error of line against schema, nil when it is valid.
func validates(t *testing.T, schema *jsonschema.Schema, line string) error {
	t.Helper()
	value, err := jsonschema.UnmarshalJSON(strings.NewReader(line))
	require.NoError(t, err)
	return schema.Validate(value)
}

// lineOf is event as a line of the stream about the publisher's repository, outside any operation for the stream's own lines.
func lineOf(t *testing.T, event domain.Event) string {
	t.Helper()
	line := domain.Line{V: domain.StreamVersion, TS: "2026-10-16T12:00:00Z", Repo: &domain.Repo{Root: "/repo", CommonDir: "/repo/.git"}, OpID: "op-1", Event: event}
	switch event.(type) {
	case domain.Ready:
		line.Repo, line.OpID = nil, ""
	case domain.Snapshot, domain.RepoAdded, domain.RepoRemoved:
		line.OpID = ""
	}
	data, err := domain.MarshalLine(line)
	require.NoError(t, err)
	return string(data)
}
