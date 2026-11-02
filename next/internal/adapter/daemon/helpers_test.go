package daemon_test

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/daemon"
	"github.com/LucasPcq/wtm/next/internal/adapter/statefile"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/run"

	"github.com/stretchr/testify/require"
)

// socketDir is a directory short enough for a unix socket, whose path macOS caps at 104 bytes.
func socketDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "wtmd")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// serve answers each connection on a new socket with answer once it read a request, and returns the socket and the requests it reads.
func serve(t *testing.T, answer string) (string, <-chan map[string]any) {
	t.Helper()
	socket := filepath.Join(socketDir(t), "wtm.sock")
	listener, err := net.Listen("unix", socket)
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	requests := make(chan map[string]any, 8)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			var request map[string]any
			_ = json.NewDecoder(conn).Decode(&request)
			requests <- request
			_, _ = conn.Write([]byte(answer + "\n"))
			_ = conn.Close()
		}
	}()

	return socket, requests
}

// unanswered returns the path of a socket no daemon listens on, the daemon's index of jobs beside it holding content, none when content is empty.
func unanswered(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	if content != "" {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "jobs.json"), []byte(content), 0o644))
	}
	return filepath.Join(dir, "wtm.sock")
}

// served is a run daemon serving the protocol on a new socket for jobs, of build version, and the number of shutdowns it was asked; a shutdown closes it.
func served(t *testing.T, jobs daemon.Jobs, version string) (string, *atomic.Int32) {
	t.Helper()
	socket := filepath.Join(socketDir(t), "wtm.sock")
	return socket, servedAt(t, socket, jobs, version)
}

// servedAt serves the protocol on socket, as served does, and returns the number of shutdowns it was asked.
func servedAt(t *testing.T, socket string, jobs daemon.Jobs, version string) *atomic.Int32 {
	t.Helper()
	// As the daemon does, the lock beside the socket is held while it serves and freed only once its listener closed, which a client waits for before it starts the next.
	lock := holdLock(t, filepath.Join(filepath.Dir(socket), "wtm.lock"))
	_ = os.Remove(socket)
	listener, err := net.Listen("unix", socket)
	require.NoError(t, err)
	stop := sync.OnceFunc(func() {
		_ = listener.Close()
		_ = lock.Close()
	})
	t.Cleanup(stop)
	shutdowns := &atomic.Int32{}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			if line, err := bufio.NewReader(conn).ReadBytes('\n'); err == nil {
				daemon.Server{Jobs: jobs, Version: version, Stop: func() { shutdowns.Add(1); stop() }}.Serve(conn, line)
			}
			_ = conn.Close()
		}
	}()
	return shutdowns
}

// holdLock takes the flock of the file at path, as a running daemon holds the lock beside its socket.
func holdLock(t *testing.T, path string) *os.File {
	t.Helper()
	lock, err := statefile.TryLock(path, domain.LockExclusive)
	require.NoError(t, err)
	return lock
}

// asked writes request on socket and returns the answers it reads until the connection ends.
func asked(t *testing.T, socket, request string) []map[string]any {
	t.Helper()
	conn, err := net.Dial("unix", socket)
	require.NoError(t, err)
	defer conn.Close()
	_, err = conn.Write([]byte(request + "\n"))
	require.NoError(t, err)
	var answers []map[string]any
	decoder := json.NewDecoder(conn)
	for {
		var a map[string]any
		if decoder.Decode(&a) != nil {
			return answers
		}
		delete(a, "pid")
		answers = append(answers, a)
	}
}

// servedConcurrently serves the protocol as served does, each connection in a goroutine of its own, as the daemon does: a follow or an attach holds its connection.
func servedConcurrently(t *testing.T, jobs daemon.Jobs) (string, *sync.WaitGroup) {
	t.Helper()
	socket := filepath.Join(socketDir(t), "wtm.sock")
	listener, err := net.Listen("unix", socket)
	require.NoError(t, err)
	var serving sync.WaitGroup
	t.Cleanup(func() {
		_ = listener.Close()
		serving.Wait()
	})
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			serving.Go(func() {
				defer conn.Close()
				if line, err := bufio.NewReader(conn).ReadBytes('\n'); err == nil {
					daemon.Server{Jobs: jobs, Version: "1.0"}.Serve(conn, line)
				}
			})
		}
	}()
	return socket, &serving
}

// starts are the specs of the starts fake got, in order.
func starts(fake *jobs) []any {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	var specs []any
	for _, req := range fake.requests {
		if spec, ok := req.(run.Spec); ok {
			specs = append(specs, spec)
		}
	}
	return specs
}
