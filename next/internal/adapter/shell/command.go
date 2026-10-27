// Package shell starts every child process of wtm-next in a group of its own, stops it the way a terminal would, and runs the hooks.
package shell

import (
	"context"
	"errors"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// The escalation of a cancelled child: SIGINT to its group, SIGTERM after
// interruptGrace, SIGKILL once its pipes had pipeGrace more to drain.
const (
	interruptGrace = time.Second
	pipeGrace      = time.Second
	killDelay      = interruptGrace + pipeGrace
	groupPoll      = 20 * time.Millisecond
)

// A Cmd is an [exec.Cmd] started in a process group of its own. Cancelling its
// context signals the whole group as a terminal's Ctrl+C would, SIGINT, then
// SIGTERM, then SIGKILL, and once it returns no member of its group is left
// running. Use Wait, Run or Output on the Cmd itself, not on the embedded
// exec.Cmd, so that group is reaped.
type Cmd struct {
	*exec.Cmd
	stop *escalation
}

// An escalation is the signals pending on one cancelled child's group, ended
// when the child returns.
type escalation struct {
	mu     sync.Mutex
	ended  bool
	pgid   int
	timers []*time.Timer
}

// Command returns a Cmd that runs name with args in a process group of its own,
// signalled as a group when ctx is cancelled.
func Command(ctx context.Context, name string, args ...string) *Cmd {
	cmd := &Cmd{Cmd: exec.CommandContext(ctx, name, args...), stop: &escalation{}}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		pgid := cmd.Process.Pid
		group := func(signal syscall.Signal) error { return ignoreGone(syscall.Kill(-pgid, signal)) }
		cmd.stop.arm(armParams{Group: pgid, Kill: func() { _ = group(syscall.SIGKILL) }, Terminate: func() { _ = group(syscall.SIGTERM) }})
		return group(syscall.SIGINT)
	}
	cmd.WaitDelay = killDelay
	return cmd
}

// Wait waits for the child as [exec.Cmd.Wait] does, then stops what is left of
// its group.
func (c *Cmd) Wait() error {
	defer c.stop.end()
	return c.Cmd.Wait()
}

// Run runs the child as [exec.Cmd.Run] does, then stops what is left of its
// group.
func (c *Cmd) Run() error {
	defer c.stop.end()
	return c.Cmd.Run()
}

// Output runs the child and returns its standard output as [exec.Cmd.Output]
// does, then stops what is left of its group.
func (c *Cmd) Output() ([]byte, error) {
	defer c.stop.end()
	return c.Cmd.Output()
}

// armParams is the group a cancelled child runs in and the signals to send it.
type armParams struct {
	Group     int
	Kill      func()
	Terminate func()
}

// arm schedules SIGTERM then SIGKILL on a cancelled child's group and makes it
// one KillCancelled stops at once; a child that already returned is left alone.
func (e *escalation) arm(params armParams) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.ended {
		return
	}

	e.pgid = params.Group
	stopping(e, params.Kill)
	e.timers = append(e.timers, time.AfterFunc(interruptGrace, params.Terminate), time.AfterFunc(killDelay, params.Kill))
}

// end cancels the pending signals once the child has returned and, if it was
// cancelled, stops the members of its group still running.
func (e *escalation) end() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ended = true
	for _, timer := range e.timers {
		timer.Stop()
	}
	defer stopped(e)

	if e.pgid == 0 || !groupAlive(e.pgid) {
		return
	}
	reap(e.pgid)
}

// reapGroup stops what is left of the group pgid once its leader returned, a member it started in the background; a group already empty is left alone.
func reapGroup(pgid int) {
	if groupAlive(pgid) {
		reap(pgid)
	}
}

// reap stops the members of a group its leader left behind: SIGTERM, then
// SIGKILL if any is still alive after interruptGrace.
func reap(pgid int) {
	_ = ignoreGone(syscall.Kill(-pgid, syscall.SIGTERM))

	deadline := time.Now().Add(interruptGrace)
	for groupAlive(pgid) && time.Now().Before(deadline) {
		time.Sleep(groupPoll)
	}

	if groupAlive(pgid) {
		_ = ignoreGone(syscall.Kill(-pgid, syscall.SIGKILL))
	}
}

// groupAlive reports whether any process of the group pgid still exists.
func groupAlive(pgid int) bool {
	return syscall.Kill(-pgid, 0) == nil
}

// ignoreGone returns nil for the error of signalling a group that no longer
// exists, err otherwise.
func ignoreGone(err error) error {
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}
