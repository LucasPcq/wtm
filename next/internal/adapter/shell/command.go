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

const (
	interruptGrace = time.Second
	pipeGrace      = time.Second
	killDelay      = interruptGrace + pipeGrace
	groupPoll      = 20 * time.Millisecond
)

type Cmd struct {
	*exec.Cmd
	stop *escalation
}

type escalation struct {
	mu     sync.Mutex
	ended  bool
	pgid   int
	timers []*time.Timer
}

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

func (c *Cmd) Wait() error {
	defer c.stop.end()
	return c.Cmd.Wait()
}

func (c *Cmd) Run() error {
	defer c.stop.end()
	return c.Cmd.Run()
}

func (c *Cmd) Output() ([]byte, error) {
	defer c.stop.end()
	return c.Cmd.Output()
}

type armParams struct {
	Group     int
	Kill      func()
	Terminate func()
}

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
	_ = ignoreGone(syscall.Kill(-e.pgid, syscall.SIGTERM))
	deadline := time.Now().Add(interruptGrace)
	for groupAlive(e.pgid) && time.Now().Before(deadline) {
		time.Sleep(groupPoll)
	}
	if groupAlive(e.pgid) {
		_ = ignoreGone(syscall.Kill(-e.pgid, syscall.SIGKILL))
	}
}

func groupAlive(pgid int) bool {
	return syscall.Kill(-pgid, 0) == nil
}

func ignoreGone(err error) error {
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}
