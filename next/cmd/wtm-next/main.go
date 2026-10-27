// Command wtm-next manages the git worktrees of a repository: it is wtm rebuilt as use-case slices over a light hexagon, working on the same repositories and state files.
//
// main.go registers each command's command line on the root; wire.go builds each adapter once and injects it; project.go finds the repository a command runs on from the directory the command line names.
package main

import (
	"os"

	bus "github.com/LucasPcq/wtm/next/internal/adapter/events"
	"github.com/LucasPcq/wtm/next/internal/adapter/shell"
	cleancli "github.com/LucasPcq/wtm/next/internal/command/clean/cli"
	createcli "github.com/LucasPcq/wtm/next/internal/command/create/cli"
	eventscli "github.com/LucasPcq/wtm/next/internal/command/events/cli"
	execcli "github.com/LucasPcq/wtm/next/internal/command/exec/cli"
	extractcli "github.com/LucasPcq/wtm/next/internal/command/extract/cli"
	listcli "github.com/LucasPcq/wtm/next/internal/command/list/cli"
	prunecli "github.com/LucasPcq/wtm/next/internal/command/prune/cli"
	resolvecli "github.com/LucasPcq/wtm/next/internal/command/resolve/cli"
	statuscli "github.com/LucasPcq/wtm/next/internal/command/status/cli"
	treecli "github.com/LucasPcq/wtm/next/internal/command/tree/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
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
		cleancli.New(streams, env, openClean, commandLine),
		prunecli.New(streams, env, openPrune, commandLine),
		execcli.New(streams, env, openExec, commandLine),
		listcli.New(streams, env, openList, commandLine),
		resolvecli.New(streams, env, openResolve, commandLine),
		statuscli.New(streams, env, openStatus, commandLine),
		treecli.New(streams, env, openTree, commandLine),
		eventscli.New(streams, env, systemStream(), commandLine),
		uiCmd(env),
		daemonCmd(),
	}
}

// daemonCmd is the hidden command that runs the event bus's daemon: wtm events starts it when none answers, and it stops once idle.
func daemonCmd() *cobra.Command {
	return &cobra.Command{
		Use:    daemonCommand,
		Short:  "Run the event bus every wtm-next process publishes to and wtm events reads",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return bus.Serve(cmd.Context(), bus.ServeParams{SocketPath: bus.SocketPath(), Idle: daemonIdle})
		},
	}
}
