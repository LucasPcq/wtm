package process

import (
	"bytes"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/testutil/socktest"
)

func startService(t *testing.T, m *Manager, job domain.JobConfig) string {
	t.Helper()
	dir := t.TempDir()
	if err := m.Start(StartParams{Job: job, WorkDir: dir}); err != nil {
		t.Fatalf("start %s: %v", job.Name, err)
	}
	t.Cleanup(func() { _ = m.StopAll() })
	return dir
}

func TestManagerResize_SetsThePTYSize(t *testing.T) {
	m := newManager()
	dir := startService(t, m, domain.JobConfig{Name: "dev", Kind: domain.JobKindService, Cmd: "sleep 30"})

	session, err := m.Attach(JobRef{Name: "dev", WorkDir: dir})
	if err != nil {
		t.Fatalf("attach: %v", err)
	}
	defer session.Release()

	assertPTYSize := func(cols int, rows int) {
		t.Helper()
		gotRows, gotCols, sizeErr := pty.Getsize(session.PTY)
		if sizeErr != nil {
			t.Fatalf("read pty size: %v", sizeErr)
		}
		if gotCols != cols || gotRows != rows {
			t.Errorf("pty is %dx%d, want %dx%d", gotCols, gotRows, cols, rows)
		}
	}

	if err := m.Resize(ResizeParams{Name: "dev", WorkDir: dir, Cols: 80, Rows: 20}); err != nil {
		t.Fatalf("resize: %v", err)
	}
	assertPTYSize(80, 20)

	// A second pane taking over resizes it again rather than being refused.
	if err := m.Resize(ResizeParams{Name: "dev", WorkDir: dir, Cols: 132, Rows: 43}); err != nil {
		t.Fatalf("second resize: %v", err)
	}
	assertPTYSize(132, 43)
}

// TestManagerResize_RefusesWhatHasNoPTY walks the jobs a pane can ask to resize
// and must be told about instead: each answers with an error naming the job,
// never a panic on a nil or closed file.
func TestManagerResize_RefusesWhatHasNoPTY(t *testing.T) {
	m := newManager()

	serviceDir := startService(t, m, domain.JobConfig{Name: "dev", Kind: domain.JobKindService, Cmd: "sleep 30"})
	launcherDir := startService(t, m, domain.JobConfig{
		Name: "compose", Kind: domain.JobKindService, Cmd: "echo up", Stop: "echo down",
	})

	stoppedDir := startService(t, m, domain.JobConfig{Name: "old", Kind: domain.JobKindService, Cmd: "sleep 30"})
	if err := m.Stop(JobRef{Name: "old", WorkDir: stoppedDir}); err != nil {
		t.Fatalf("stop: %v", err)
	}

	taskDir := t.TempDir()
	script := filepath.Join(taskDir, "slow.sh")
	if err := os.WriteFile(script, []byte("sleep 30\n"), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}
	taskDone := make(chan struct{})
	go func() {
		defer close(taskDone)
		_ = m.Start(StartParams{
			Job:     domain.JobConfig{Name: "migrate", Kind: domain.JobKindTask, Cmd: "sh " + script},
			WorkDir: taskDir,
		})
	}()
	waitForJob(t, m, "migrate", func(ManagedJob) bool { return true })
	t.Cleanup(func() {
		_ = m.Stop(JobRef{Name: "migrate", WorkDir: taskDir})
		<-taskDone
	})

	cases := []struct {
		name    string
		params  ResizeParams
		wantMsg string
	}{
		{"job inconnu", ResizeParams{Name: "ghost", WorkDir: serviceDir, Cols: 80, Rows: 20}, "job ghost not found"},
		{"job arrêté", ResizeParams{Name: "old", WorkDir: stoppedDir, Cols: 80, Rows: 20}, "job old is not running"},
		{"launcher détaché", ResizeParams{Name: "compose", WorkDir: launcherDir, Cols: 80, Rows: 20}, "job compose is detached"},
		{"task sur un pipe", ResizeParams{Name: "migrate", WorkDir: taskDir, Cols: 80, Rows: 20}, "job migrate runs on a pipe"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := m.Resize(c.params)
			if err == nil {
				t.Fatalf("expected an error, got nil")
			}
			if !strings.Contains(err.Error(), c.wantMsg) {
				t.Errorf("error = %q, want it to contain %q", err, c.wantMsg)
			}
		})
	}
}

// TestManagerResize_RefusesASizeAWinsizeCannotHold walks the sizes that must
// not reach the ioctl. The out-of-range ones are the reason the check lives at
// the truncation point: 65536 columns fit an int and wrap to a zero-width
// window, which is how a job ends up permanently in plain log mode.
func TestManagerResize_RefusesASizeAWinsizeCannotHold(t *testing.T) {
	m := newManager()
	dir := startService(t, m, domain.JobConfig{Name: "dev", Kind: domain.JobKindService, Cmd: "sleep 30"})

	session, err := m.Attach(JobRef{Name: "dev", WorkDir: dir})
	if err != nil {
		t.Fatalf("attach: %v", err)
	}
	defer session.Release()

	if err := m.Resize(ResizeParams{Name: "dev", WorkDir: dir, Cols: 120, Rows: 40}); err != nil {
		t.Fatalf("resize: %v", err)
	}

	cases := []struct {
		name    string
		params  ResizeParams
		wantMsg string
	}{
		{"taille nulle", ResizeParams{Name: "dev", WorkDir: dir}, "job dev: invalid size 0x0"},
		{"taille négative", ResizeParams{Name: "dev", WorkDir: dir, Cols: -1, Rows: 20}, "job dev: invalid size -1x20"},
		{"colonnes hors bornes", ResizeParams{Name: "dev", WorkDir: dir, Cols: 65536, Rows: 24}, "job dev: invalid size 65536x24"},
		{"lignes hors bornes", ResizeParams{Name: "dev", WorkDir: dir, Cols: 80, Rows: 70000}, "job dev: invalid size 80x70000"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := m.Resize(c.params)
			if err == nil {
				t.Fatalf("expected an error, got nil")
			}
			if !strings.Contains(err.Error(), c.wantMsg) {
				t.Errorf("error = %q, want it to contain %q", err, c.wantMsg)
			}

			rows, cols, sizeErr := pty.Getsize(session.PTY)
			if sizeErr != nil {
				t.Fatalf("read pty size: %v", sizeErr)
			}
			if cols != 120 || rows != 40 {
				t.Errorf("pty is %dx%d, want the refused resize to have left it at 120x40", cols, rows)
			}
		})
	}
}

func TestDaemonHandleResize_ReportsTheManagerVerdict(t *testing.T) {
	d := &daemonServer{manager: newManager()}
	dir := startService(t, d.manager, domain.JobConfig{Name: "dev", Kind: domain.JobKindService, Cmd: "sleep 30"})

	var ok bytes.Buffer
	d.handleResize(replyEncoder{enc: json.NewEncoder(&ok)}, Request{Action: ActionResize, Name: "dev", WorkDir: dir, Cols: 90, Rows: 24})
	if got := decodeResponse(t, ok.Bytes()); got.Status != StatusOK {
		t.Errorf("status = %s (%s), want ok", got.Status, got.Message)
	}

	var ko bytes.Buffer
	d.handleResize(replyEncoder{enc: json.NewEncoder(&ko)}, Request{Action: ActionResize, Name: "ghost", WorkDir: dir, Cols: 90, Rows: 24})
	got := decodeResponse(t, ko.Bytes())
	if got.Status != StatusError {
		t.Fatalf("status = %s, want error", got.Status)
	}
	if !strings.Contains(got.Message, "job ghost not found") {
		t.Errorf("message = %q, want it to name the unknown job", got.Message)
	}
}

// TestClientResize_TravelsOnItsOwnConnection pins the contract the run view
// depends on: the size is a request of its own, not bytes pushed into the
// attach stream, which by then belongs to the job's stdin.
func TestClientResize_TravelsOnItsOwnConnection(t *testing.T) {
	socket := socktest.Path(t)
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	requests := make(chan Request, 1)
	go serveVersionedOK(listener, requests, Response{Status: StatusOK, Version: domain.Version})

	client := NewClient(socket)
	if err := client.Resize(ResizeParams{Name: "dev", WorkDir: "/work/feat", Cols: 80, Rows: 20}); err != nil {
		t.Fatalf("resize: %v", err)
	}

	select {
	case req := <-requests:
		if req.Action != ActionResize {
			t.Errorf("action = %s, want %s", req.Action, ActionResize)
		}
		if req.Name != "dev" || req.WorkDir != "/work/feat" || req.Cols != 80 || req.Rows != 20 {
			t.Errorf("request = %+v, want dev@/work/feat 80x20", req)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the daemon never received a resize request")
	}
}

func TestClientResize_SurfacesTheDaemonError(t *testing.T) {
	socket := socktest.Path(t)
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	go serveVersionedOK(listener, nil, Response{Status: StatusError, Version: domain.Version, Message: "job dev not found"})

	err = NewClient(socket).Resize(ResizeParams{Name: "dev", Cols: 80, Rows: 20})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "job dev not found") {
		t.Errorf("error = %q, want the daemon message", err)
	}
}

// serveVersionedOK stands in for the daemon across as many connections as the
// client opens: a mutating request is preceded by a listing, which is how the
// client settles the version question before the daemon acts on anything.
func serveVersionedOK(listener net.Listener, requests chan<- Request, answer Response) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		var req Request
		if decodeErr := json.NewDecoder(conn).Decode(&req); decodeErr != nil {
			conn.Close()
			return
		}
		if req.Action == ActionList {
			_ = json.NewEncoder(conn).Encode(Response{Status: StatusOK, Version: domain.Version})
			conn.Close()
			continue
		}
		if requests != nil {
			requests <- req
		}
		_ = json.NewEncoder(conn).Encode(answer)
		conn.Close()
	}
}

func decodeResponse(t *testing.T, raw []byte) Response {
	t.Helper()
	var resp Response
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("decode response %q: %v", raw, err)
	}
	return resp
}
