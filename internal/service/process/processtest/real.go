package processtest

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/service/process"
)

// RealDaemon runs the actual daemon in this process on socket, for a test about
// what the daemon itself does — the event broker — rather than about which
// jobs it reports. Its state lives under a HOME of its own. Stop ends it the way
// `run daemon stop` would, and the test's cleanup does if nothing else did.
func RealDaemon(t *testing.T, socket string) (stop func()) {
	t.Helper()
	home, err := os.MkdirTemp("/tmp", "wtmh")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(home) })
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	exited := make(chan error, 1)
	go func() { exited <- process.RunDaemon(process.DaemonParams{SocketPath: socket}) }()
	deadline := time.Now().Add(2 * time.Second)
	for !process.IsDaemonRunning(socket) {
		if time.Now().After(deadline) {
			t.Fatalf("daemon never answered on %s", socket)
		}
		time.Sleep(5 * time.Millisecond)
	}

	stopped := false
	stop = func() {
		if stopped {
			return
		}
		stopped = true
		_, _ = process.NewClient(socket).SendUnchecked(process.Request{Action: process.ActionShutdown})
		select {
		case <-exited:
		case <-time.After(3 * time.Second):
			t.Errorf("daemon on %s did not exit", socket)
		}
	}
	t.Cleanup(stop)
	return stop
}
