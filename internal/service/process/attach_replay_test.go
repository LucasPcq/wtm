package process

import (
	"encoding/json"
	"io"
	"net"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"

	"github.com/LucasPcq/wtm/internal/testutil/socktest"
)

// TestAttachKeepsTheHistorySentWithTheAcceptance pins what the run view's
// replay depends on: the daemon answers the create command and writes the job's
// buffered history right behind it, so a single read carries both.
func TestAttachKeepsTheHistorySentWithTheAcceptance(t *testing.T) {
	const history = "web:dev ready in 412ms\nrebuilt in 97ms\n"

	socket := socktest.Path(t)
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer conn.Close()
		var req Request
		if decErr := json.NewDecoder(conn).Decode(&req); decErr != nil {
			return
		}
		accepted, marshalErr := json.Marshal(Response{Status: StatusOK, Version: domain.Version, Message: "attached"})
		if marshalErr != nil {
			return
		}
		// One write, as the daemon does: the acceptance and the replay land in
		// the same read on the client side.
		_, _ = conn.Write(append(append(accepted, '\n'), history...))
		time.Sleep(50 * time.Millisecond)
	}()

	conn, err := NewClient(socket).Attach(AttachParams{Name: "web", WorkDir: "/w", Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("attach: %v", err)
	}
	defer conn.Close()

	if deadlineErr := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); deadlineErr != nil {
		t.Fatalf("set deadline: %v", deadlineErr)
	}

	buf := make([]byte, len(history))
	n, err := io.ReadFull(conn, buf)
	if err != nil {
		t.Fatalf("read replay: %v (got %q)", err, buf[:n])
	}
	if got := string(buf); got != history {
		t.Errorf("replay = %q, want %q", got, history)
	}
}
