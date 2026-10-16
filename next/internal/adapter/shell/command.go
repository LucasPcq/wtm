// Package shell starts every child process of wtm-next in a group of its own, stops it the way a terminal would, and runs the hooks.
package shell

import (
	"context"
	"errors"
	"os/exec"
	"syscall"
	"time"
)

const (
	interruptGrace = time.Second
	pipeGrace      = time.Second
	killDelay      = interruptGrace + pipeGrace
)

func Command(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		pgid := cmd.Process.Pid
		group := func(signal syscall.Signal) error { return ignoreGone(syscall.Kill(-pgid, signal)) }
		stopping(func() { _ = group(syscall.SIGKILL) })
		time.AfterFunc(killDelay, func() { _ = group(syscall.SIGKILL) })
		time.AfterFunc(interruptGrace, func() { _ = group(syscall.SIGTERM) })
		return group(syscall.SIGINT)
	}
	cmd.WaitDelay = killDelay
	return cmd
}

func ignoreGone(err error) error {
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}
