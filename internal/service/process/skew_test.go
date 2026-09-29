package process

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

// oldDaemon answers like a v0.27 daemon: no version, no PID, and no idea what
// a shutdown request is.
type oldDaemon struct {
	listener net.Listener
	// version is empty for a daemon predating the handshake.
	version string
	jobs    []domain.JobInfo
	mu      sync.Mutex
	actions []RequestAction
}

func skewSocket(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "wtms")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return filepath.Join(dir, domain.DaemonSocketName)
}

func serveOld(t *testing.T, socket string, jobs []domain.JobInfo) *oldDaemon {
	t.Helper()
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	d := &oldDaemon{listener: listener, jobs: jobs}
	t.Cleanup(func() { listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go d.serve(conn)
		}
	}()
	return d
}

func (d *oldDaemon) serve(conn net.Conn) {
	defer conn.Close()
	var req Request
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		return
	}
	d.mu.Lock()
	d.actions = append(d.actions, req.Action)
	d.mu.Unlock()
	enc := json.NewEncoder(conn)
	switch req.Action {
	case ActionList:
		_ = enc.Encode(map[string]any{"status": "ok", "jobs": d.jobs, "version": d.version})
	case ActionStop, ActionStopAll:
		_ = enc.Encode(map[string]any{"status": "ok"})
	default:
		_ = enc.Encode(map[string]any{"status": "error", "message": "unknown action: " + string(req.Action)})
	}
}

func (d *oldDaemon) received(action RequestAction) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, got := range d.actions {
		if got == action {
			return true
		}
	}
	return false
}

func TestAnOlderDaemonCanBeListedAndStopped(t *testing.T) {
	socket := skewSocket(t)
	old := serveOld(t, socket, []domain.JobInfo{{Name: "web", Status: domain.JobStatusRunning, WorkDir: "/w"}})
	client := NewClient(socket)

	resp, err := client.Send(Request{Action: ActionList})
	if err != nil || len(resp.Jobs) != 1 {
		t.Fatalf("list = %+v, %v; want the older daemon's job", resp.Jobs, err)
	}
	if _, err := client.Send(Request{Action: ActionStop, Name: "web", WorkDir: "/w"}); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if _, err := client.Send(Request{Action: ActionStopAll}); err != nil {
		t.Fatalf("stop all: %v", err)
	}
	if !old.received(ActionStop) || !old.received(ActionStopAll) {
		t.Error("the older daemon never received the stops")
	}
}

func TestAnOlderDaemonIsStillRefusedAStart(t *testing.T) {
	socket := skewSocket(t)
	serveOld(t, socket, nil)
	job := domain.JobConfig{Name: "web", Kind: domain.JobKindService, Cmd: "true"}

	_, err := NewClient(socket).Send(Request{Action: ActionStart, Job: &job})
	if !errors.Is(err, domain.ErrDaemonVersionMismatch) {
		t.Fatalf("error = %v, want a version mismatch", err)
	}
}

func TestDaemonPeerPIDReadsTheListeningProcess(t *testing.T) {
	socket := skewSocket(t)
	serveOld(t, socket, nil)

	pid, err := DaemonPeerPID(socket)
	if err != nil {
		t.Fatalf("peer pid: %v", err)
	}
	if pid != os.Getpid() {
		t.Errorf("pid = %d, want %d", pid, os.Getpid())
	}
}

// replaceOnTerminate stands in for the signal and for the fork: the old daemon
// goes, and one of this build takes the socket, as `wtm daemon` would.
func replaceOnTerminate(t *testing.T, old *oldDaemon, socket string) *[]int {
	t.Helper()
	var signalled []int
	previousTerminate, previousSpawn := terminate, spawnDaemon
	terminate = func(pid int) error {
		signalled = append(signalled, pid)
		return old.listener.Close()
	}
	spawnDaemon = func(DaemonParams) error {
		current := serveOld(t, socket, nil)
		current.version = domain.Version
		return nil
	}
	t.Cleanup(func() { terminate, spawnDaemon = previousTerminate, previousSpawn })
	return &signalled
}

func TestShutdownSignalsADaemonThatPredatesTheRequest(t *testing.T) {
	socket := skewSocket(t)
	old := serveOld(t, socket, nil)
	var signalled []int
	previous := terminate
	terminate = func(pid int) error {
		signalled = append(signalled, pid)
		old.listener.Close()
		return nil
	}
	t.Cleanup(func() { terminate = previous })

	if err := Shutdown(socket); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if len(signalled) != 1 || signalled[0] != os.Getpid() {
		t.Errorf("signalled = %v, want the process holding the socket", signalled)
	}
}

func TestEnsureCurrentDaemonReplacesAnIdleOlderDaemon(t *testing.T) {
	socket := skewSocket(t)
	old := serveOld(t, socket, nil)
	signalled := replaceOnTerminate(t, old, socket)

	if err := EnsureCurrentDaemon(DaemonParams{SocketPath: socket}); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if len(*signalled) != 1 {
		t.Errorf("the idle older daemon was not replaced")
	}
}

func TestEnsureCurrentDaemonRefusesAnOlderDaemonHoldingJobs(t *testing.T) {
	socket := skewSocket(t)
	old := serveOld(t, socket, []domain.JobInfo{
		{Name: "web", Status: domain.JobStatusRunning},
		{Name: "db", Status: domain.JobStatusRunning},
	})
	signalled := replaceOnTerminate(t, old, socket)

	err := EnsureCurrentDaemon(DaemonParams{SocketPath: socket})
	if !errors.Is(err, domain.ErrDaemonVersionMismatch) {
		t.Fatalf("error = %v, want a version mismatch", err)
	}
	for _, want := range []string{"2 job", "wtm run daemon restart"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message %q does not say %q", err, want)
		}
	}
	if len(*signalled) != 0 {
		t.Error("a daemon holding jobs was replaced")
	}
}
