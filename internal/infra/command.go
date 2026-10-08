package infra

import (
	"context"
	"errors"
	"os/exec"
	"syscall"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// Command is exec.CommandContext with a cancellation a child can clean up
// after, as it would after a Ctrl-C at the terminal: SIGINT, SIGTERM a grace
// later, and the kill once WaitDelay runs out — to the child and to what it
// started.
func Command(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	if shielded(ctx) {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}
	cmd.Cancel = func() error {
		group, _ := syscall.Getpgid(cmd.Process.Pid)
		family := rules.Descendants(rules.DescendantsParams{Links: processTable(), Root: cmd.Process.Pid, Group: group})
		tree := func(signal syscall.Signal) error {
			err := cmd.Process.Signal(signal)
			for _, pid := range rules.StillInGroup(rules.StillInGroupParams{Links: processTable(), PIDs: family, Group: group}) {
				_ = syscall.Kill(pid, signal)
			}
			return err
		}
		stopping(func() { _ = tree(syscall.SIGKILL) })
		return escalate(tree)
	}
	cmd.WaitDelay = killDelay
	return cmd
}

// processTable is read to reach what the child started — git's ssh or
// upload-pack — which git does not stop when it is stopped itself. Only what
// is still in the child's group is ever signalled: a daemon git started to
// outlive it has left it.
func processTable() []domain.ProcessLink {
	table, err := exec.Command("ps", "-Ao", "pid=,ppid=,pgid=").Output()
	if err != nil {
		return nil
	}
	return rules.ParseProcessLinks(string(table))
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
		stopping(func() { _ = group(syscall.SIGKILL) })
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
