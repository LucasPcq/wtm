package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/infra"
)

const shieldHelperEnv = "WTM_TEST_SHIELD_HELPER"

type interruptHelper struct {
	cmd   *exec.Cmd
	lines *bufio.Scanner
}

func startHelper(t *testing.T, test string) interruptHelper {
	t.Helper()
	helper := exec.Command(os.Args[0], "-test.run=^"+test+"$")
	helper.Env = append(os.Environ(), shieldHelperEnv+"=1")
	stdout, err := helper.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := helper.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = helper.Process.Kill() })
	return interruptHelper{cmd: helper, lines: bufio.NewScanner(stdout)}
}

func (h interruptHelper) expect(t *testing.T, want string) string {
	t.Helper()
	if !h.lines.Scan() {
		t.Fatalf("helper ended before saying %q", want)
	}
	if want != "" && h.lines.Text() != want {
		t.Fatalf("helper said %q, want %q", h.lines.Text(), want)
	}
	return h.lines.Text()
}

func (h interruptHelper) diesOnSIGINT(t *testing.T) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- h.cmd.Wait() }()
	select {
	case err := <-done:
		var exitErr *exec.ExitError
		status, ok := syscall.WaitStatus(0), false
		if errors.As(err, &exitErr) {
			status, ok = exitErr.Sys().(syscall.WaitStatus)
		}
		if !ok || !status.Signaled() || status.Signal() != syscall.SIGINT {
			t.Fatalf("helper ended with %v, want SIGINT", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the second interrupt did not end the run")
	}
}

// A second Ctrl-C does not cut a shielded step — a worktree half removed — off:
// the process ends once it is done, not before.
func TestASecondInterruptWaitsForAShieldedStep(t *testing.T) {
	if os.Getenv(shieldHelperEnv) != "" {
		ctx, stop := interruptible()
		defer stop()
		_, release := infra.Shield(ctx)
		fmt.Println("armed")
		<-ctx.Done()
		fmt.Println("cancelled")
		time.Sleep(time.Second)
		fmt.Println("released")
		release()
		time.Sleep(time.Minute)
		os.Exit(0)
	}

	helper := startHelper(t, "TestASecondInterruptWaitsForAShieldedStep")
	helper.expect(t, "armed")
	_ = helper.cmd.Process.Signal(os.Interrupt)
	helper.expect(t, "cancelled")
	_ = helper.cmd.Process.Signal(os.Interrupt)
	helper.expect(t, "released")
	helper.diesOnSIGINT(t)
}

// A child that ignores the interrupt is killed with the process that gave up
// waiting for it, instead of outliving it.
func TestASecondInterruptTakesTheChildItWasStoppingWithIt(t *testing.T) {
	if os.Getenv(shieldHelperEnv) != "" {
		ctx, stop := interruptible()
		defer stop()
		child := infra.Command(ctx, "sh", "-c", `trap '' INT TERM; echo $$; exec sleep 30`)
		child.Stdout = os.Stdout
		_ = child.Run()
		time.Sleep(time.Minute)
		os.Exit(0)
	}

	helper := startHelper(t, "TestASecondInterruptTakesTheChildItWasStoppingWithIt")
	pid, err := strconv.Atoi(helper.expect(t, ""))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
	time.Sleep(100 * time.Millisecond)
	_ = helper.cmd.Process.Signal(os.Interrupt)
	time.Sleep(100 * time.Millisecond)
	_ = helper.cmd.Process.Signal(os.Interrupt)
	helper.diesOnSIGINT(t)

	deadline := time.Now().Add(500 * time.Millisecond)
	for syscall.Kill(pid, 0) == nil {
		if time.Now().After(deadline) {
			t.Fatal("the child that ignored the interrupt outlived the run")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
