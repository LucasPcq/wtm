package infra

import (
	"context"
	"errors"
	"os/exec"
	"syscall"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

// Command is exec.CommandContext with a cancellation a child can clean up
// after, as it would after a Ctrl-C at the terminal: SIGINT, SIGTERM a grace
// later, and the kill once WaitDelay runs out.
func Command(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Cancel = func() error {
		return escalate(func(signal syscall.Signal) error { return cmd.Process.Signal(signal) })
	}
	cmd.WaitDelay = killDelay
	return cmd
}

// GroupCommand is Command for a child that runs in its own process group —
// out of reach of the terminal's own Ctrl-C — so every signal goes to the whole
// group, ending on SIGKILL: a shell's grandchildren would otherwise outlive it.
func GroupCommand(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		pgid := cmd.Process.Pid
		group := func(signal syscall.Signal) error { return ignoreGone(syscall.Kill(-pgid, signal)) }
		time.AfterFunc(killDelay, func() { _ = group(syscall.SIGKILL) })
		return escalate(group)
	}
	cmd.WaitDelay = killDelay
	return cmd
}

// killDelay is also how long Wait holds on to a pipe a finished child left
// open (a job it backgrounded): WaitDelay bounds both.
const killDelay = domain.SubprocessInterruptGrace + domain.ExecPipeGrace

func escalate(signal func(syscall.Signal) error) error {
	time.AfterFunc(domain.SubprocessInterruptGrace, func() { _ = signal(syscall.SIGTERM) })
	return signal(syscall.SIGINT)
}

func ignoreGone(err error) error {
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}
