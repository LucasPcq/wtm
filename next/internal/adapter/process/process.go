// Package process runs the processes of the run daemon's jobs: a service under a terminal of its own, anything else through a pipe, each in a process group of its own; it stops a group the way a terminal's hangup would, and probes and reaps the groups a daemon that died left behind.
package process

import (
	"errors"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"

	"github.com/LucasPcq/wtm/next/internal/adapter/shell"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
)

const (
	shellPath = "/bin/sh"
	// TUI tools decide at start whether to draw by the terminal's size: a 0x0 one makes them fall back to plain logs for good.
	terminalRows = 40
	terminalCols = 120
	// drainGrace bounds the wait for a child's output once it exited: a process it left in the background may hold the terminal or the pipe open for ever.
	drainGrace = time.Second
	chunkSize  = 32 << 10
)

// A Child is a job's process, the leader of its own group.
type Child struct {
	cmd      *exec.Cmd
	terminal bool
	output   *os.File
	exited   chan struct{}
	drained  chan struct{}
	code     int
	once     sync.Once
}

// Spawn starts command: under a terminal of its own, in a session of its own,
// when command.Terminal, else with its stdout and stderr on one pipe, in a
// process group of its own; its output goes to command.Output until it ends.
func Spawn(command run.Command) (*Child, error) {
	cmd := exec.Command(shellPath, "-c", command.Line)
	cmd.Dir = command.Dir
	cmd.Env = shell.Environ(command.Unset, command.Env)

	output, err := start(cmd, command.Terminal)
	if err != nil {
		return nil, err
	}

	child := &Child{cmd: cmd, terminal: command.Terminal, output: output, exited: make(chan struct{}), drained: make(chan struct{})}
	go child.drain(command.Output)
	go child.reap()

	return child, nil
}

// start starts cmd and returns the end of its output this process reads.
func start(cmd *exec.Cmd, terminal bool) (*os.File, error) {
	if terminal {
		return pty.StartWithSize(cmd, &pty.Winsize{Rows: terminalRows, Cols: terminalCols})
	}

	read, write, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	cmd.Stdout, cmd.Stderr = write, write
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	err = cmd.Start()
	// The child holds its own copy: closing this one lets the read end see the end of the output once the child closes its.
	_ = write.Close()
	if err != nil {
		_ = read.Close()
		return nil, err
	}

	return read, nil
}

func (c *Child) drain(output func([]byte)) {
	defer close(c.drained)
	buf := make([]byte, chunkSize)
	for {
		n, err := c.output.Read(buf)
		if n > 0 && output != nil {
			output(append([]byte(nil), buf[:n]...))
		}
		if err != nil {
			return
		}
	}
}

func (c *Child) reap() {
	err := c.cmd.Wait()
	c.code = exitCode(err)
	close(c.exited)
}

// exitCode is the code a child's wait returned err for: its exit status, -1
// when a signal killed it.
func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if code, exited := shell.ExitCode(err); exited {
		return code
	}

	return -1
}

// PID returns the child's process id, its group's id too.
func (c *Child) PID() int {
	return c.cmd.Process.Pid
}

// Wait waits for the child to exit and its output to end, at most a second
// after its exit, and returns its exit code, -1 when a signal killed it. What
// is left of its group runs on: a launcher's stack is meant to.
func (c *Child) Wait() int {
	<-c.exited
	select {
	case <-c.drained:
	case <-time.After(drainGrace):
	}
	c.once.Do(func() { _ = c.output.Close() })

	return c.code
}

// Stop stops the child's whole group as a terminal's hangup would: SIGTERM,
// then SIGKILL once grace passed with the child still running, and SIGKILL to
// any member left once it exited. It returns once no member of the group is
// alive, a couple of seconds at most: a member its leader left is reaped by
// the init process, in its own time.
func (c *Child) Stop(grace time.Duration) {
	signalGroup(c.PID(), syscall.SIGTERM)
	select {
	case <-c.exited:
	case <-time.After(grace):
		signalGroup(c.PID(), syscall.SIGKILL)
		<-c.exited
	}
	signalGroup(c.PID(), syscall.SIGKILL)
	deadline := time.Now().Add(reapGrace)
	for groupAlive(c.PID()) && time.Now().Before(deadline) {
		time.Sleep(groupPoll)
	}
}

// errNoTerminal is the error of typing into, or sizing, a child run through a pipe.
var errNoTerminal = errors.New("the job runs under no terminal")

// Write types p into the child's terminal.
func (c *Child) Write(p []byte) (int, error) {
	if !c.terminal {
		return 0, errNoTerminal
	}
	return c.output.Write(p)
}

// Resize sizes the child's terminal, cols by rows.
func (c *Child) Resize(cols, rows int) error {
	if !c.terminal {
		return errNoTerminal
	}
	return pty.Setsize(c.output, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
}

// signalGroup sends signal to the group pgid, a group gone or not ours
// ignored; a group id of 0 or 1 is refused before any syscall, kill(-1)
// signalling everything the user owns.
func signalGroup(pgid int, signal syscall.Signal) {
	if pgid > 1 {
		_ = syscall.Kill(-pgid, signal)
	}
}
