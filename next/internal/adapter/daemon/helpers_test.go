package daemon_test

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"

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

// index writes the daemon's index of jobs with content and returns its path.
func index(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "jobs.json")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}
