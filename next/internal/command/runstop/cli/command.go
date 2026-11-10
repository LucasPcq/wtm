// Package cli is run stop's command line: its flags, its form, its output and its JSON, over run stop's use case.
package cli

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/command/runstop"
	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/cobra"
)

// Use is how the command line spells run stop after the program's name.
const Use = "run stop"

const (
	stopShort = "Stop one job in worktrees"
	stopLong  = "Stop one job wtm's daemon runs in each worktree named, or in the one you stand in;\n" +
		"a worktree it is not up in is left as it is. Several worktrees stop at once.\n" +
		"\n" +
		"A shared service a worktree holds is released: it runs on for the worktrees still holding it, and\n" +
		"stops with its last hold. A stack stops by its stop command. A job that crashed is cleared.\n" +
		"A worktree removed while its jobs ran is named by its path."
	stopExample = `  # The api of the worktree you stand in
  wtm run stop --job api

  # The shared database's hold of two worktrees
  wtm run stop feat/login fix/typo --job db

  # For a script or an agent
  wtm run stop feat/login --job api --yes --output json`

	yesUsage = "Run unattended: no question, no confirmation (the worktree you stand in unless given; --job is required)"
)

type stopCommand struct {
	io   uicli.Run
	form *stopForm
}

// New returns run stop's command, a subcommand of run: its flags and arguments become a runstop.Request, answered in the wizard when the user can see and answer it, else run unattended; it draws the outcome on streams, or the v2 JSON envelope with --output json, its follow-ups spelled by commandLine.
func New(streams *output.Streams, env uicli.Env, open func(ctx context.Context, dir string) (runstop.UseCase, error), commandLine output.CommandLine) *cobra.Command {
	c := &stopCommand{io: uicli.Run{Command: runstop.Name, Streams: streams, Env: env, CommandLine: commandLine, Flags: stopFlags}, form: newForm(open)}
	cmd := &cobra.Command{Use: "stop [worktree...]", Short: stopShort, Long: stopLong, Example: stopExample, Args: cobra.ArbitraryArgs, RunE: c.run}
	c.form.Bind(cmd, &c.io)

	return cmd
}

func (c *stopCommand) run(cmd *cobra.Command, args []string) error {
	return c.form.Launch(cmd, args, c.form.Conclude)
}

// Line spells a follow-up that names run stop, from run stop's schema, as wtm-next runs it; none when its answers are not run stop's.
func Line(followUp domain.FollowUp) []string {
	return newForm(nil).Spell(uicli.Program+" "+Use, followUp)
}
