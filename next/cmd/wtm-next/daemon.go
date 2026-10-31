package main

import (
	"bufio"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
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
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
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
		Probe: names.Probe,
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

// commandRestartDaemon is the follow-up that restarts the run daemon, as a follow-up names it.
const commandRestartDaemon = "run daemon restart"

// runDaemonCmd is run daemon, the commands on the run daemon itself: restart, which replaces a daemon of another build, or of the legacy wtm, by one of this build.
func runDaemonCmd(streams *output.Streams) *cobra.Command {
	cmd := &cobra.Command{Use: "daemon", Short: "Manage the run daemon", Args: cobra.NoArgs}
	cmd.AddCommand(restartCmd(streams, func() restartParams {
		runs := runDaemon()
		return restartParams{Jobs: runs.Jobs, Shutdown: runs.Shutdown, Ensure: runs.Ensure}
	}))
	return cmd
}

// restartCmd is run daemon restart over the daemon daemon gives, asking on streams.
func restartCmd(streams *output.Streams, daemon func() restartParams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restart",
		Short: "Restart the run daemon, as this build",
		Long:  "Replace a run daemon of another build, or of the legacy wtm, by one of this build; a daemon of this build is left alone. Every service the daemon runs itself stops with it, in every repository, a shared one included; its detached stacks (a service with a stop command, docker compose up -d) and the holds worktrees have on shared services are handed over to the new one through its index. It says which jobs stop and which are kept, then asks; --yes restarts without asking.",
		Args:  cobra.NoArgs,
	}
	yes := cmd.Flags().BoolP("yes", "y", false, "Restart without asking")
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		failing := cli.Run{Command: commandRestartDaemon, Streams: streams}
		p := daemon()
		quiet, _ := cmd.Flags().GetBool(cli.FlagQuiet)
		p.In, p.Out, p.Err, p.CanAsk, p.Yes, p.Quiet = streams.In, cmd.OutOrStdout(), cmd.ErrOrStderr(), streams.CanPrompt(), *yes, quiet
		if failure := domain.AsError(domain.AsErrorParams{Err: restartDaemon(cmd.Context(), p)}); failure != nil {
			return failing.FailWith(failure, nil)
		}
		return nil
	}
	return cmd
}

// restartParams are the daemon a restart replaces, through what reads its jobs, stops it and starts one of this build; where it asks, and says what it found (Err), where it says it restarted (Out, silent when Quiet); whether it can ask; and whether --yes answered. Its failures are *domain.Error, by kind: unattended without --yes invalid, declined cancelled.
type restartParams struct {
	Jobs     func(ctx context.Context) ([]run.Job, error)
	Shutdown func(ctx context.Context) error
	Ensure   func(ctx context.Context) error
	In       io.Reader
	Out      io.Writer
	Err      io.Writer
	CanAsk   bool
	Yes      bool
	Quiet    bool
}

// The codes of a restart not made: unattended without --yes, or declined.
const (
	codeRestartUnconfirmed = "run.restart_unconfirmed"
	codeRestartDeclined    = "run.restart_declined"
)

// restartDaemon replaces a daemon of another build or protocol by one of this build, once the user saw what stops and agreed; a daemon of this build, or none, is left as it is, and one that died leaving jobs in its index is followed by a new one.
func restartDaemon(ctx context.Context, p restartParams) error {
	jobs, err := p.Jobs(ctx)
	skewed, isSkewed := errors.AsType[*run.Skewed](err)
	refused, _ := errors.AsType[*run.Error](err)
	protocol := refused != nil && refused.Code == run.CodeSkew
	switch {
	case errors.Is(err, run.ErrUnknown):
		if err := p.Ensure(ctx); err != nil {
			return err
		}
		p.said("A run daemon of this build runs: the stacks and claims of its index are kept, the services a daemon that died left are stopped")
		return nil
	case err != nil && !isSkewed && !protocol:
		return err
	case err == nil:
		p.said("The run daemon is of this build, or none runs: nothing to restart")
		return nil
	}

	// What stops is said where the question is asked; --yes asks none, and --quiet then says none.
	if !p.Quiet || !p.Yes {
		p.found(foundParams{Jobs: jobs, Skewed: skewed, Protocol: protocol})
	}
	if !p.Yes {
		if !p.CanAsk {
			return &domain.Error{Kind: domain.KindInvalid, Code: codeRestartUnconfirmed, Message: "nothing restarted: pass --yes to restart it unattended"}
		}
		fmt.Fprint(p.Err, "  Restart it? [y/N] ")
		answer, _ := bufio.NewReader(p.In).ReadString('\n')
		if !strings.EqualFold(strings.TrimSpace(answer), "y") {
			return &domain.Error{Kind: domain.KindCancelled, Code: codeRestartDeclined, Message: "nothing restarted"}
		}
	}
	if err := p.Shutdown(ctx); err != nil {
		return err
	}
	if err := p.Ensure(ctx); err != nil {
		return err
	}
	p.said("The run daemon restarted as wtm-next " + version)
	return nil
}

// foundParams are what a restart found of the daemon it replaces: its jobs, its skew, or that it speaks another protocol.
type foundParams struct {
	Jobs     []run.Job
	Skewed   *run.Skewed
	Protocol bool
}

// found says on Err what a restart found: which jobs stop and which are kept.
func (p restartParams) found(f foundParams) {
	output.Frame(p.Err, func(w io.Writer) {
		if f.Protocol {
			output.Message(w, "The run daemon speaks another protocol: its jobs can't be read.")
			output.Message(w, "Stops:  every service it runs itself")
			output.Message(w, "Kept:   its stacks, in its index")
			return
		}
		stops, kept := restartReach(f.Jobs)
		output.Message(w, "The run daemon is "+daemonOf(f.Skewed)+".")
		output.Message(w, "Stops:  "+cmp.Or(strings.Join(stops, ", "), "nothing"))
		output.Message(w, "Kept:   "+cmp.Or(strings.Join(kept, ", "), "nothing"))
	})
}

// said writes what the restart did on Out, framed, unless Quiet.
func (p restartParams) said(done string) {
	if p.Quiet {
		return
	}
	output.Frame(p.Out, func(w io.Writer) { output.SuccessLine(w, done) })
}

// daemonOf names the daemon of skewed: the legacy wtm's, or of another build.
func daemonOf(skewed *run.Skewed) string {
	if skewed.Skew == run.SkewLegacy {
		return "the legacy wtm's"
	}
	return "of another build (" + cmp.Or(skewed.Daemon, "older") + ")"
}

// restartReach are the jobs up a restart stops, the services the daemon runs itself, and those it keeps, the detached stacks and the holds on shared services, each by its name and worktree.
func restartReach(jobs []run.Job) (stops, kept []string) {
	for _, job := range jobs {
		if !job.Up {
			continue
		}
		named := job.Name + " (" + job.WorkDir + ")"
		if job.Hold == run.HoldOwn {
			stops = append(stops, named)
			continue
		}
		kept = append(kept, named)
	}
	return stops, kept
}
