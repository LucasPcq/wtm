package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/next/internal/adapter/config"
	"github.com/LucasPcq/wtm/next/internal/adapter/daemon"
	bus "github.com/LucasPcq/wtm/next/internal/adapter/events"
	"github.com/LucasPcq/wtm/next/internal/adapter/joblog"
	"github.com/LucasPcq/wtm/next/internal/adapter/process"
	"github.com/LucasPcq/wtm/next/internal/adapter/proxy"
	"github.com/LucasPcq/wtm/next/internal/adapter/shell"
	rundaemon "github.com/LucasPcq/wtm/next/internal/daemon"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/ui/cli"
)

// daemonCommand is the command line that runs the daemon in a process of its own.
const daemonCommand = "daemon"

// flagProxyPort is the port the daemon's proxy serves named URLs on, 0 turning it off; the global config's when not given.
const flagProxyPort = "proxy-port"

// daemonIdle is how long the daemon waits with no subscriber, no request and no process of a job before it stops.
const daemonIdle = 30 * time.Second

// errDaemonInTest refuses to start a daemon from a test: the executable is the test binary, which would run the whole suite again as "daemon".
var errDaemonInTest = errors.New("no daemon is started from a test")

// daemonCmd is the hidden command that runs the daemon of the machine: the run daemon, which holds the jobs of every worktree, and the event bus every wtm-next process publishes to, on one socket. A client that needs one starts it; it stops once idle, leaving detached stacks to the next.
func daemonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    daemonCommand,
		Short:  "Run the daemon that holds the jobs and relays the event bus",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			port, problem := proxyPort(cmd)
			return serveDaemon(cmd.Context(), daemonParams{Socket: bus.SocketPath(), ProxyPort: port, ProxyProblem: problem, Stderr: cmd.ErrOrStderr()})
		},
	}
	cmd.Flags().Int(flagProxyPort, 0, "Port the run proxy serves named job URLs on (0 turns it off); the global config's when not given")
	return cmd
}

// proxyPort returns the port the daemon's proxy is to serve on: the flag's, else the global config's; 0 with why when that config cannot be read: a daemon refusing to start would refuse every job.
func proxyPort(cmd *cobra.Command) (int, error) {
	if cmd.Flags().Changed(flagProxyPort) {
		port, _ := cmd.Flags().GetInt(flagProxyPort)
		return port, nil
	}
	global, err := config.LoadGlobal(config.GlobalPath())
	if err != nil {
		return 0, err
	}
	return global.ProxyPort(), nil
}

// daemonParams is the daemon to serve: on Socket, its proxy on ProxyPort (0 for none) or ProxyProblem why it has none, what goes wrong said on Stderr, the daemon's log.
type daemonParams struct {
	Socket       string
	ProxyPort    int
	ProxyProblem error
	Stderr       io.Writer
}

// serveDaemon runs the daemon until ctx is done or it idles out. Once it holds the lock, it empties its log, binds its proxy, then takes over the jobs its index names, so a stack taken over is served again, then serves the run daemon's requests and the bus's; it closes its proxy before it lets the lock go, so the next daemon binds the same port. A proxy that cannot bind is said on stderr, and every job answers on its port.
func serveDaemon(ctx context.Context, params daemonParams) error {
	socket := params.Socket
	relay := bus.NewRelay()
	names := proxy.New()
	defer names.Close()
	jobs := rundaemon.New(rundaemon.Params{
		Spawn:   spawn,
		Orphans: process.Orphans{},
		Index:   &daemon.Index{Path: daemon.IndexPath(socket)},
		OpenLog: openLog,
		ReadLog: joblog.Last,
		Exists:  dirExists,
		Publish: relay.Publish,
		Routes:  names,
		// Probed on every answer, never cached: the redirection is installed and removed while the daemon runs.
		Probe: proxy.Probe,
	})
	return bus.Serve(ctx, bus.ServeParams{
		SocketPath: socket,
		Idle:       daemonIdle,
		Relay:      relay,
		Ready: func() {
			daemon.EmptyLog(socket)
			if params.ProxyProblem != nil {
				fmt.Fprintf(params.Stderr, "run proxy: %v — jobs keep their own ports\n", params.ProxyProblem)
			}
			if params.ProxyPort > 0 {
				if err := names.Listen(params.ProxyPort); err != nil {
					fmt.Fprintf(params.Stderr, "run proxy: cannot serve on port %d (%v) — jobs keep their own ports\n", params.ProxyPort, err)
				}
			}
			jobs.Adopt()
		},
		Other: func(conn net.Conn, line []byte, stop func()) {
			daemon.Server{Jobs: jobs, Version: version, Stop: stop}.Serve(conn, line)
		},
		Busy: jobs.Busy,
		Stopping: func() {
			jobs.Shutdown()
			_ = names.Close()
		},
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

// openLog opens a job's log, as the run daemon's port takes it.
func openLog(file run.LogFile) (rundaemon.Log, error) {
	log, err := joblog.Open(file)
	if err != nil {
		return nil, err
	}
	return log, nil
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
