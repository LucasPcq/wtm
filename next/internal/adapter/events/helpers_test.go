package events

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

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
