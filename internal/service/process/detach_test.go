package process

import (
	"errors"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

// A long-lived client — `wtm events`, `wtm ui` — is the parent of the daemon it
// forks. Unreaped, a stopped daemon stays a zombie, which kill(pid, 0) still
// finds: `run daemon stop` then waits 30s for an exit that already happened.
func TestADetachedChildIsReapedOnceItExits(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	if err := startDetached(cmd); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("pid %d is still there: the exited child was never reaped", pid)
}
