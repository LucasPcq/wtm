// Package cli is daemon stop's command line: its form, its output and its JSON, over daemon stop's use case.
package cli

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/command/daemonstop"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/cobra"
)

// Use is how the command line spells daemon stop after the program's name.
const Use = "daemon stop"

const (
	stopShort = "Stop wtm's daemon, its stacks left running"
	stopLong  = "Stop wtm's daemon, the one process of this machine that runs the jobs of every\n" +
		"repository. Every service it runs itself stops with it, in every repository, a shared\n" +
		"one included, and the worktrees holding that one let it go. Its stacks (a service with\n" +
		"a stop command, docker compose up -d) run on, and the next daemon takes them over.\n\n" +
		"It asks first when a service it runs would stop; --yes stops without asking. The\n" +
		"next command that needs the daemon starts it again; `wtm daemon restart` stops it\n" +
		"and starts it again with its jobs."
	stopExample = `  wtm daemon stop

  # For a script or an agent
  wtm daemon stop --yes --output json`

	yesUsage = "Stop without asking"
)

type stopCommand struct {
	io   uicli.Run
	form *stopForm
}

// New returns daemon stop's command, a subcommand of daemon: it previews the shutdown, asks to confirm it when a service would stop and the user can see and answer it, then draws the outcome on streams, or the v2 JSON envelope with --output json, its follow-ups spelled by commandLine.
func New(streams *output.Streams, env uicli.Env, open func(ctx context.Context, dir string) (daemonstop.UseCase, error), commandLine output.CommandLine) *cobra.Command {
	c := &stopCommand{io: uicli.Run{Command: daemonstop.Name, Streams: streams, Env: env, CommandLine: commandLine, Flags: stopFlags}, form: newForm(open)}
	cmd := &cobra.Command{Use: "stop", Short: stopShort, Long: stopLong, Example: stopExample, Args: cobra.NoArgs, RunE: c.run}
	c.form.Bind(cmd, &c.io)

	return cmd
}

func (c *stopCommand) run(cmd *cobra.Command, args []string) error {
	return c.form.Launch(cmd, args, c.form.Conclude)
}
