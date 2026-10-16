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

type fakeDaemon struct {
	socket    string
	commonDir string
	requests  chan string
}

func listen(t *testing.T) *fakeDaemon {
	t.Helper()
	return listenAnswering(t, `{"status":"ok"}`)
}

func listenAnswering(t *testing.T, answer string) *fakeDaemon {
	t.Helper()
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

func publisherOn(t *testing.T, daemon *fakeDaemon) Publisher {
	t.Helper()
	return NewPublisher(PublisherParams{Root: "/repo", CommonDir: daemon.commonDir, SocketPath: daemon.socket, OpID: "op-1", CorrelationID: "popup-1", Now: fixedNow})
}

func payloadOf(t *testing.T, request string) string {
	t.Helper()
	var envelope struct {
		Payload json.RawMessage `json:"payload"`
	}
	require.NoError(t, json.Unmarshal([]byte(request), &envelope))
	return string(envelope.Payload)
}

func withoutPayload(t *testing.T, request string) string {
	t.Helper()
	var envelope map[string]any
	require.NoError(t, json.Unmarshal([]byte(request), &envelope))
	envelope["payload"] = map[string]any{}
	data, err := json.Marshal(envelope)
	require.NoError(t, err)
	return string(data)
}

func decoded(t *testing.T, payload string) map[string]any {
	t.Helper()
	var event map[string]any
	require.NoError(t, json.Unmarshal([]byte(payload), &event))
	return event
}

func symlink(target, link string) error {
	return os.Symlink(target, link)
}
