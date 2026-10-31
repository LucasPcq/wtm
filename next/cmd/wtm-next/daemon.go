package main

import (
	"context"
	"errors"
	"net"
	"os"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/next/internal/adapter/daemon"
	bus "github.com/LucasPcq/wtm/next/internal/adapter/events"
	"github.com/LucasPcq/wtm/next/internal/adapter/process"
	"github.com/LucasPcq/wtm/next/internal/adapter/shell"
	rundaemon "github.com/LucasPcq/wtm/next/internal/daemon"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/ui/cli"
)

// daemonCommand is the command line that runs the daemon in a process of its own.
const daemonCommand = "daemon"

// daemonIdle is how long the daemon waits with no subscriber, no request and no process of a job before it stops.
const daemonIdle = 30 * time.Second

// errDaemonInTest refuses to start a daemon from a test: the executable is the test binary, which would run the whole suite again as "daemon".
var errDaemonInTest = errors.New("no daemon is started from a test")

// daemonCmd is the hidden command that runs the daemon of the machine: the run daemon, which holds the jobs of every worktree, and the event bus every wtm-next process publishes to, on one socket. A client that needs one starts it; it stops once idle, leaving detached stacks to the next.
func daemonCmd() *cobra.Command {
	return &cobra.Command{
		Use:    daemonCommand,
		Short:  "Run the daemon that holds the jobs and relays the event bus",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return serveDaemon(cmd.Context(), bus.SocketPath())
		},
	}
}

// serveDaemon runs the daemon on socket until ctx is done or it idles out: it takes over the jobs its index names, then serves the run daemon's requests and the bus's.
func serveDaemon(ctx context.Context, socket string) error {
	relay := bus.NewRelay()
	jobs := rundaemon.New(rundaemon.Params{
		Spawn:   spawn,
		Orphans: process.Orphans{},
		Index:   &daemon.Index{Path: daemon.IndexPath(socket)},
		Exists:  dirExists,
		Publish: relay.Publish,
	})
	return bus.Serve(ctx, bus.ServeParams{
		SocketPath: socket,
		Idle:       daemonIdle,
		Relay:      relay,
		Ready: func() {
			daemon.EmptyLog(socket)
			jobs.Adopt()
		},
		Other: func(conn net.Conn, line []byte, stop func()) {
			daemon.Server{Jobs: jobs, Version: version, Stop: stop}.Serve(conn, line)
		},
		Busy:     jobs.Busy,
		Stopping: jobs.Shutdown,
	})
}

// spawn starts a job's process, as the run daemon's port takes it.
func spawn(command run.Command) (rundaemon.Process, error) {
	child, err := process.Spawn(command)
	if err != nil {
		return nil, err
	}
	return child, nil
}

// dirExists reports whether the directory at path is there.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// runDaemon is the run daemon of this machine as a command asks it: on the bus's socket, started from this binary when one is needed.
func runDaemon() daemon.Daemon {
	return daemon.Daemon{SocketPath: bus.SocketPath(), Version: version, StartDaemon: startDaemon}
}

// startDaemon starts this binary's daemon in a session of its own, with this process's environment but its correlation ID, which belongs to the one command that started it, its standard error to the daemon's log, where a start that failed says why.
func startDaemon() error {
	if testing.Testing() {
		return errDaemonInTest
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	return shell.Detached(shell.DetachedParams{Name: exe, Args: []string{daemonCommand}, Env: shell.Environ([]string{cli.CorrelationIDEnv}, nil), Log: daemon.LogPath(bus.SocketPath())})
}
