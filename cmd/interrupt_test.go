package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

const interruptHelperEnv = "WTM_TEST_INTERRUPT_HELPER"

// The first interrupt cancels the run; the second is the default one, so a run
// slow to unwind can still be killed outright.
func TestFirstInterruptCancelsSecondKills(t *testing.T) {
	if os.Getenv(interruptHelperEnv) != "" {
		ctx, stop := interruptible()
		defer stop()
		fmt.Println("armed")
		<-ctx.Done()
		fmt.Println("cancelled")
		time.Sleep(time.Minute)
		os.Exit(0)
	}

	helper := exec.Command(os.Args[0], "-test.run=^TestFirstInterruptCancelsSecondKills$")
	helper.Env = append(os.Environ(), interruptHelperEnv+"=1")
	stdout, err := helper.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := helper.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = helper.Process.Kill() })
	lines := bufio.NewScanner(stdout)

	expect := func(want string) {
		t.Helper()
		if !lines.Scan() || lines.Text() != want {
			t.Fatalf("helper said %q, want %q", lines.Text(), want)
		}
	}
	expect("armed")
	_ = helper.Process.Signal(os.Interrupt)
	expect("cancelled")
	time.Sleep(100 * time.Millisecond)
	_ = helper.Process.Signal(os.Interrupt)

	done := make(chan error, 1)
	go func() { done <- helper.Wait() }()
	select {
	case err := <-done:
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("helper ended with %v, want death by signal", err)
		}
		status, ok := exitErr.Sys().(syscall.WaitStatus)
		if !ok || !status.Signaled() || status.Signal() != syscall.SIGINT {
			t.Fatalf("helper ended with %v, want SIGINT", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the second interrupt did not kill the run")
	}
}
