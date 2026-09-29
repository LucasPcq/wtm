// Package socktest hands tests a Unix socket path short enough to bind, and a
// stand-in for whatever listens on it.
package socktest

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
)

// shortTempBase is used instead of t.TempDir(), which embeds the test name
// under /var/folders on macOS and overruns sun_path (104 bytes), failing the
// bind with EINVAL.
const shortTempBase = "/tmp"

// Path returns a daemon socket path under a directory removed when the test ends.
func Path(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp(shortTempBase, "wtm")
	if err != nil {
		t.Fatalf("socket dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return filepath.Join(dir, "wtm.sock")
}

// Serve answers every connection to path with handle's reply to the one JSON
// request it carries — a daemon reduced to its protocol, for a test that must
// not start the real one.
func Serve(t *testing.T, path string, handle func(json.RawMessage) any) {
	t.Helper()
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			var req json.RawMessage
			if json.NewDecoder(conn).Decode(&req) == nil {
				_ = json.NewEncoder(conn).Encode(handle(req))
			}
			conn.Close()
		}
	}()
}
