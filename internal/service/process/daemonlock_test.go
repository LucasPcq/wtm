package process

import (
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

// A second daemon on the same socket used to remove the first one's socket and
// listen in its place: two daemons, two indexes written over each other, and a
// first one nobody can reach any more.
func TestASecondDaemonLeavesTheFirstOneServing(t *testing.T) {
	daemon := idleDaemon(t, time.Minute, time.Second)
	t.Cleanup(func() {
		_ = Shutdown(daemon.socket)
		<-daemon.exited
	})

	second := make(chan error, 1)
	go func() { second <- RunDaemon(DaemonParams{SocketPath: daemon.socket}) }()

	select {
	case err := <-second:
		if !errors.Is(err, domain.ErrDaemonRunning) {
			t.Errorf("second daemon returned %v, want ErrDaemonRunning", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the second daemon is still running beside the first")
	}
	if !IsDaemonRunning(daemon.socket) {
		t.Error("the first daemon's socket stopped answering")
	}
}

// Between a daemon closing its socket and its process exiting, its foreground
// jobs are in their grace period and it still writes the index. A daemon
// spawned in that window would be one of two.
func TestEnsureDaemonWaitsForAStoppingDaemonToExit(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	socket := skewSocket(t)
	stopping, err := acquireDaemonLock(socket)
	if err != nil {
		t.Fatalf("lock: %v", err)
	}
	released := atomic.Bool{}
	time.AfterFunc(300*time.Millisecond, func() {
		released.Store(true)
		stopping.Close()
	})

	spawnedEarly := atomic.Bool{}
	previous := spawnDaemon
	spawnDaemon = func(params DaemonParams) error {
		spawnedEarly.Store(!released.Load())
		go func() { _ = RunDaemon(params) }()
		return nil
	}
	t.Cleanup(func() { spawnDaemon = previous })

	if err := EnsureDaemon(DaemonParams{SocketPath: socket}); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if spawnedEarly.Load() {
		t.Error("a daemon was spawned while the previous one had not exited")
	}
	if err := Shutdown(socket); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}
