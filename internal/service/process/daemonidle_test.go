package process

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

// testDaemon is a real daemon on a real socket. `exited` carries what RunDaemon
// returned, which is the whole subject here: in production that return is the
// process going away, and any request still in flight dies with it.
type testDaemon struct {
	socket string
	exited chan error
}

func idleDaemon(t *testing.T, idle, budget time.Duration) testDaemon {
	t.Helper()
	t.Setenv("HOME", t.TempDir())

	previousIdle, previousBudget := daemonIdleTimeout, daemonNamespaceBudget
	daemonIdleTimeout, daemonNamespaceBudget = idle, budget
	t.Cleanup(func() { daemonIdleTimeout, daemonNamespaceBudget = previousIdle, previousBudget })

	// Not t.TempDir(): a unix socket path is capped at ~104 bytes, and the
	// per-test temp directory alone is longer than that.
	dir, err := os.MkdirTemp("/tmp", "wtmd")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	daemon := testDaemon{socket: filepath.Join(dir, domain.DaemonSocketName), exited: make(chan error, 1)}
	go func() { daemon.exited <- RunDaemon(DaemonParams{SocketPath: daemon.socket}) }()

	deadline := time.Now().Add(2 * time.Second)
	for !IsDaemonRunning(daemon.socket) {
		if time.Now().After(deadline) {
			t.Fatalf("daemon never answered on its socket (%s)", daemon.socket)
		}
		time.Sleep(5 * time.Millisecond)
	}
	return daemon
}

// answer sends the request and reports which came first: the daemon's answer, or
// the daemon exiting under it.
func (d testDaemon) answer(t *testing.T, req Request) Response {
	t.Helper()
	type outcome struct {
		resp Response
		err  error
	}
	answered := make(chan outcome, 1)
	go func() {
		resp, err := NewClient(d.socket).Send(req)
		answered <- outcome{resp: resp, err: err}
	}()

	select {
	case err := <-d.exited:
		t.Fatalf("daemon exited while a request was in flight (%v); a client reads that as a closed socket", err)
		return Response{}
	case got := <-answered:
		if got.err != nil {
			t.Fatalf("send: %v", got.err)
		}
		return got.resp
	}
}

// sharedRequest is a shared job whose own command exits at once — the shape of a
// `docker compose up -d` — so nothing is left Running while its namespace command
// works. That is the state the idle watcher reads.
func sharedRequest(t *testing.T, create string) Request {
	t.Helper()
	return Request{
		Action:  ActionStart,
		WorkDir: t.TempDir(),
		Env:     map[string]string{domain.EnvWorktree: "feat-a", domain.EnvOrdinal: "1"},
		Job: &domain.JobConfig{
			Name: "db", Kind: domain.JobKindService, Cmd: "true",
			Scope:     domain.JobScopeShared,
			Namespace: &domain.JobNamespaceConfig{Name: "app_{worktree}", Create: create},
		},
		Shared: &domain.SharedJobContext{
			WorkDir: t.TempDir(),
			Env:     map[string]string{domain.EnvWorktree: "main", domain.EnvOrdinal: "0"},
		},
	}
}

// A namespace command outliving the idle timeout must still be answered. The
// daemon auto-exits on a count of running jobs, and a shared service that
// launches detached leaves none — so the request carving out its namespace was
// killed by its own daemon.
func TestDaemonIdleExitSparesARequestInFlight(t *testing.T) {
	daemon := idleDaemon(t, 50*time.Millisecond, time.Second)

	resp := daemon.answer(t, sharedRequest(t, "sleep 0.4"))
	if resp.Status != StatusOK {
		t.Fatalf("status = %s (%s), want ok", resp.Status, resp.Message)
	}
}

// The error a namespace command reports is the whole point of running it, and it
// only reaches the caller if the daemon is still there to send it.
func TestDaemonIdleExitSparesAFailingNamespace(t *testing.T) {
	daemon := idleDaemon(t, 50*time.Millisecond, 100*time.Millisecond)

	resp := daemon.answer(t, sharedRequest(t, "sleep 0.2; echo already exists >&2; exit 1"))
	if resp.Status != StatusError {
		t.Fatalf("status = %s (%s), want error", resp.Status, resp.Message)
	}
	if !strings.Contains(resp.Message, "already exists") {
		t.Errorf("message = %q, want the command's own output", resp.Message)
	}
}

// Sparing a request in flight must not cost the auto-exit: a daemon holding
// nothing still goes away on its own.
func TestDaemonIdleExitsWithNothingInFlight(t *testing.T) {
	daemon := idleDaemon(t, 50*time.Millisecond, time.Second)

	select {
	case <-daemon.exited:
	case <-time.After(2 * time.Second):
		t.Fatal("daemon still running, want it exited on idle")
	}
	if _, err := os.Stat(daemon.socket); !os.IsNotExist(err) {
		t.Errorf("socket still on disk, want it removed on exit")
	}
}
