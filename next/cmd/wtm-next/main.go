// Command wtm-next manages the git worktrees of a repository: it is wtm rebuilt as use-case slices over a light hexagon, working on the same repositories and state files.
//
// main.go registers each command's command line on the root; wire.go builds each adapter once and injects it; project.go finds the repository a command runs on from the directory the command line names.
package main

import (
	"os"

	"github.com/LucasPcq/wtm/next/internal/adapter/shell"
	checkoutcli "github.com/LucasPcq/wtm/next/internal/command/checkout/cli"
	cleancli "github.com/LucasPcq/wtm/next/internal/command/clean/cli"
	createcli "github.com/LucasPcq/wtm/next/internal/command/create/cli"
	eventscli "github.com/LucasPcq/wtm/next/internal/command/events/cli"
	execcli "github.com/LucasPcq/wtm/next/internal/command/exec/cli"
	extractcli "github.com/LucasPcq/wtm/next/internal/command/extract/cli"
	fastforwardcli "github.com/LucasPcq/wtm/next/internal/command/fastforward/cli"
	listcli "github.com/LucasPcq/wtm/next/internal/command/list/cli"
	prunecli "github.com/LucasPcq/wtm/next/internal/command/prune/cli"
	reparentcli "github.com/LucasPcq/wtm/next/internal/command/reparent/cli"
	resolvecli "github.com/LucasPcq/wtm/next/internal/command/resolve/cli"
	rundowncli "github.com/LucasPcq/wtm/next/internal/command/rundown/cli"
	runlogscli "github.com/LucasPcq/wtm/next/internal/command/runlogs/cli"
	runstartcli "github.com/LucasPcq/wtm/next/internal/command/runstart/cli"
	runstopcli "github.com/LucasPcq/wtm/next/internal/command/runstop/cli"
	runupcli "github.com/LucasPcq/wtm/next/internal/command/runup/cli"
	statuscli "github.com/LucasPcq/wtm/next/internal/command/status/cli"
	synccli "github.com/LucasPcq/wtm/next/internal/command/sync/cli"
	treecli "github.com/LucasPcq/wtm/next/internal/command/tree/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
	"github.com/LucasPcq/wtm/next/internal/ui/runview"
	"github.com/spf13/cobra"
)

// version is set at build time with -ldflags "-X main.version=…".
var version = "dev"

func main() {
	streams := output.System()
	env := cli.ReadEnv()
	os.Exit(cli.Main(cli.Params{
		Version:       version,
		Streams:       streams,
		Args:          os.Args[1:],
		Commands:      commands(streams, env),
		KillCancelled: shell.KillCancelled,
		AwaitShields:  shell.AwaitShields,
	}))
}

// commands are the command lines of every command wtm-next runs, on streams in env, their follow-ups spelled by commandLine.
func commands(streams *output.Streams, env cli.Env) []*cobra.Command {
	return []*cobra.Command{
		createcli.New(streams, env, openCreate, commandLine),
		extractcli.New(streams, env, openExtract, commandLine),
		checkoutcli.New(streams, env, openCheckout, commandLine),
		cleancli.New(streams, env, openClean, commandLine),
		prunecli.New(streams, env, openPrune, commandLine),
		fastforwardcli.New(streams, env, openFastForward, commandLine),
		synccli.New(streams, env, openSync, commandLine),
		execcli.New(streams, env, openExec, commandLine),
		reparentcli.New(streams, env, openReparent, commandLine),
		listcli.New(streams, env, openList, commandLine),
		resolvecli.New(streams, env, openResolve, commandLine),
		statuscli.New(streams, env, openStatus, commandLine),
		treecli.New(streams, env, openTree, commandLine),
		eventscli.New(streams, env, systemStream(), commandLine),
		runCmd(runupcli.New(streams, env, openRunUp, commandLine, liveView()), runlogscli.New(streams, env, openRunLogs, commandLine, runlogscli.View{Daemon: runDaemon(), Open: openURL}), rundowncli.New(streams, env, openRunDown, commandLine), runstartcli.New(streams, env, openRunStart, commandLine, liveView()), runstopcli.New(streams, env, openRunStop, commandLine), runDaemonCmd(streams)),
		uiCmd(env),
		daemonCmd(),
	}
}

// liveView is where run up and run start show the jobs they started: the run daemon of this machine, never started for it, and the browser.
func liveView() runview.View {
	return runview.View{Daemon: runDaemon(), Open: openURL, Stop: stopJob, Restart: restartJob, Again: startAgain, Show: showJob}
}

// runCmd is run, the parent of the commands that start, watch and stop the jobs of run.toml.
func runCmd(commands ...*cobra.Command) *cobra.Command {
	cmd := &cobra.Command{Use: "run", Short: "Start, watch and stop the jobs of run.toml", Args: cobra.NoArgs}
	cmd.AddCommand(commands...)
	return cmd
}
