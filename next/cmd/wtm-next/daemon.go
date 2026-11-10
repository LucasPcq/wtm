package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/next/internal/adapter/config"
	"github.com/LucasPcq/wtm/next/internal/adapter/daemon"
	bus "github.com/LucasPcq/wtm/next/internal/adapter/events"
	"github.com/LucasPcq/wtm/next/internal/adapter/joblog"
	"github.com/LucasPcq/wtm/next/internal/adapter/oplock"
	"github.com/LucasPcq/wtm/next/internal/adapter/process"
	"github.com/LucasPcq/wtm/next/internal/adapter/proxy"
	"github.com/LucasPcq/wtm/next/internal/adapter/shell"
	"github.com/LucasPcq/wtm/next/internal/command/daemonrestart"
	"github.com/LucasPcq/wtm/next/internal/command/daemonstatus"
	"github.com/LucasPcq/wtm/next/internal/command/daemonstop"
	"github.com/LucasPcq/wtm/next/internal/command/daemonstopjobs"
	core "github.com/LucasPcq/wtm/next/internal/daemon"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

// serveArgs are the arguments that run the daemon in a process of its own.
var serveArgs = []string{"daemon", "serve"}

// flagProxyPort is the port the daemon's proxy serves named URLs on, 0 turning it off; the global config's when not given.
const flagProxyPort = "proxy-port"

// daemonIdle is how long the daemon waits with no subscriber, no request and no process of a job before it stops.
const daemonIdle = 30 * time.Second

// errDaemonInTest refuses to start a daemon from a test: the executable is the test binary, which would run the whole suite again as the daemon.
var errDaemonInTest = errors.New("no daemon is started from a test")

// daemonCmd is daemon, the commands on wtm's daemon itself, which runs the jobs of every repository of this machine, serves their named addresses and carries the event bus: its own subcommands, and the hidden serve that runs it.
func daemonCmd(streams *output.Streams, commands ...*cobra.Command) *cobra.Command {
	cmd := &cobra.Command{Use: "daemon", Short: "Inspect, stop or restart wtm's daemon, which runs the jobs of every repository", Args: cobra.NoArgs}
	cmd.AddCommand(commands...)
	cmd.AddCommand(daemonServeCmd(streams))
	return cmd
}

// daemonServeCmd is the hidden command that runs wtm's daemon, which holds the jobs of every worktree and the event bus every wtm-next process publishes to, on one socket. A client that needs one starts it; it stops once idle, leaving detached stacks to the next. Another daemon holding its socket is a conflict, said in the daemon's log.
func daemonServeCmd(streams *output.Streams) *cobra.Command {
	cmd := &cobra.Command{
		Use:    serveArgs[1],
		Short:  "Run wtm's daemon",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			port, problem := proxyPort(cmd)
			socket := daemon.SocketPath()
			err := serveDaemon(cmd.Context(), daemonParams{Socket: socket, ProxyPort: port, ProxyProblem: problem, Stderr: cmd.ErrOrStderr()})
			if failure := serveFailure(err, socket); failure != nil {
				failing := cli.Run{Command: "daemon serve", Streams: streams}
				return failing.FailWith(failure, nil)
			}
			return nil
		},
	}
	cmd.Flags().Int(flagProxyPort, 0, "Port the proxy serves named job URLs on (0 turns it off); the global config's when not given")
	return cmd
}

// serveFailure is why a daemon serving socket stopped, nil when it idled out: another daemon holding its lock is a conflict, daemon.running.
func serveFailure(err error, socket string) *domain.Error {
	if errors.Is(err, daemon.ErrRunning) {
		return &domain.Error{Kind: domain.KindConflict, Code: run.CodeRunning, Message: "wtm's daemon already runs on " + socket + ": this one stops"}
	}
	return domain.AsError(domain.AsErrorParams{Err: err})
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

// serveDaemon runs the daemon until ctx is done or it idles out. Once it holds the lock, it empties its log, binds its proxy, then takes over the jobs its index names, so a stack taken over is served again, then serves wtm's daemon's requests and the bus's; it closes its proxy before it lets the lock go, so the next daemon binds the same port. A proxy that cannot bind is said on stderr, and every job answers on its port.
func serveDaemon(ctx context.Context, params daemonParams) error {
	socket := params.Socket
	relay := bus.NewRelay()
	names := proxy.New()
	defer names.Close()
	jobs := core.New(core.Params{
		Spawn:   spawn,
		Orphans: process.Orphans{},
		Index:   &daemon.Index{Path: daemon.IndexPath(socket)},
		OpenLog: openLog,
		ReadLog: joblog.Last,
		Exists:  dirExists,
		Publish: relay.Publish,
		Routes:  names,
		// Probed on every answer, never cached: the redirection is installed and removed while the daemon runs.
		Probe: names.Probe,
	})
	return daemon.Serve(ctx, daemon.ServeParams{
		SocketPath: socket,
		Idle:       daemonIdle,
		Ready: func() {
			daemon.EmptyLog(socket)
			if params.ProxyProblem != nil {
				fmt.Fprintf(params.Stderr, "proxy: %v — jobs keep their own ports\n", params.ProxyProblem)
			}
			if params.ProxyPort > 0 {
				if err := names.Listen(params.ProxyPort); err != nil {
					fmt.Fprintf(params.Stderr, "proxy: cannot serve on port %d (%v) — jobs keep their own ports\n", params.ProxyPort, err)
				}
			}
			jobs.Adopt()
		},
		Bus: relay.Serve,
		Answer: func(conn net.Conn, line []byte, stop func()) {
			daemon.Server{Jobs: jobs, Version: version, Stop: stop}.Serve(conn, line)
		},
		Busy: jobs.Busy,
		Stopping: func() {
			relay.Close()
			jobs.Shutdown()
			_ = names.Close()
		},
	})
}

// spawn starts a job's process, as wtm's daemon's port takes it.
func spawn(command run.Command) (core.Process, error) {
	child, err := process.Spawn(command)
	if err != nil {
		return nil, err
	}
	return child, nil
}

// openLog opens a job's log, as wtm's daemon's port takes it.
func openLog(file run.LogFile) (core.Log, error) {
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

// openDaemonStatus is daemon status over wtm's daemon of this machine, whatever the directory: the daemon is the machine's one.
func openDaemonStatus(context.Context, string) (daemonstatus.UseCase, error) {
	return daemonstatus.UseCase{Daemon: wtmDaemon().Status, Version: version, Restart: daemonRestart()}, nil
}

// openDaemonRestart is daemon restart over wtm's daemon of this machine, whatever the directory, the new daemon of this build.
func openDaemonRestart(context.Context, string) (daemonrestart.UseCase, error) {
	return daemonrestart.UseCase{Operations: machineOperations(), Daemon: wtmDaemon(), Shield: shell.Shield, Exists: dirExists, Version: version}, nil
}

// openDaemonStopJobs is daemon stop-jobs over wtm's daemon of this machine, whatever the directory, a repository it narrows to found by its main worktree.
func openDaemonStopJobs(context.Context, string) (daemonstopjobs.UseCase, error) {
	return daemonstopjobs.UseCase{Operations: machineOperations(), Daemon: wtmDaemon(), RepoRoot: mainWorktree}, nil
}

// openDaemonStop is daemon stop over wtm's daemon of this machine, whatever the directory.
func openDaemonStop(context.Context, string) (daemonstop.UseCase, error) {
	return daemonstop.UseCase{Operations: machineOperations(), Daemon: wtmDaemon()}, nil
}

// machineOperations runs the operations on wtm's daemon, which is no repository's: recorded in the daemon's own directory, publishing no event, since the bus files every event under a repository.
func machineOperations() operation.Service {
	dir := ""
	if socket := daemon.SocketPath(); socket != "" {
		dir = filepath.Dir(socket)
	}
	return operation.Service{Locker: locker{oplock.Locks{StateDir: dir}}, Publisher: unpublished{}, NewOpID: newOpID, Now: time.Now}
}

// unpublished publishes nothing of an operation.
type unpublished struct{}

// Open returns a report that drops every event.
func (unpublished) Open(domain.Operation) domain.Report {
	return func(domain.Event) {}
}

// frozenIndex is the daemon's index of jobs when a newer wtm wrote it, "" otherwise.
func frozenIndex() string {
	if !wtmDaemon().IndexFrozen() {
		return ""
	}
	return daemon.IndexPath(daemon.SocketPath())
}

// wtmDaemon is wtm's daemon of this machine as a command asks it: on the bus's socket, started from this binary when one is needed.
func wtmDaemon() daemon.Daemon {
	return daemon.Daemon{SocketPath: daemon.SocketPath(), Version: version, StartDaemon: startDaemon}
}

// startDaemon starts this binary's daemon in a session of its own, with this process's environment but its correlation ID, which belongs to the one command that started it, its standard error to the daemon's log, where a start that failed says why; a test that restarts a daemon puts one of its own in its place.
var startDaemon = func() error {
	if testing.Testing() {
		return errDaemonInTest
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	return shell.Detached(shell.DetachedParams{Name: exe, Args: serveArgs, Env: shell.Environ([]string{cli.CorrelationIDEnv}, nil), Log: daemon.LogPath(daemon.SocketPath())})
}
