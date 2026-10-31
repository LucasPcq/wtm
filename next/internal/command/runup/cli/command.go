// Package cli is run up's command line: its flags, its form, its output and its JSON, over run up's use case.
package cli

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/command/runup"
	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/cobra"
)

// Use is how the command line spells run up after the program's name.
const Use = "run up"

const (
	upShort = "Start a profile's jobs"
	upLong  = "Start every job of a profile, in its order, in each worktree named, or in the one you stand in.\n" +
		"It starts --profile, else run.toml's default profile, else its only one; a run.toml with no\n" +
		"profile starts every job. Several worktrees start at once, each on its own: one that fails\n" +
		"leaves the others running, and the run exits 1.\n" +
		"\n" +
		"A task runs to its end before the next job starts, and its failure stops that worktree's start;\n" +
		"a service is launched and left running. A shared service runs once, in the main checkout, and\n" +
		"each worktree holds it. Once the jobs are up, each port they declare is checked, and a service\n" +
		"that ended since fails the start (--no-probe and run.toml's port_probe_timeout skip the check).\n" +
		"\n" +
		"When other worktrees run jobs, it asks whether to stop them first, unless --exclusive,\n" +
		"--parallel or run.toml's concurrency answers; --yes keeps them. A job whose touches reach data\n" +
		"the worktree does not own is refused unless --force."
	upExample = `  # The default profile, in the worktree you stand in
  wtm run up

  # Two worktrees, another profile
  wtm run up feat/login fix/typo --profile backend

  # For a script or an agent, with the v2 JSON envelope
  wtm run up feat/login --yes --output json`

	yesUsage = "Run unattended: no question, no confirmation (the worktree you stand in and run.toml's default profile unless given; the other worktrees' jobs keep running unless --exclusive)"
)

type upCommand struct {
	io   uicli.Run
	form *upForm
}

// New returns run up's command, a subcommand of run: its flags and arguments become a runup.Request, answered in the wizard when the user can see and answer it, else run unattended; it draws the outcome on streams, or the v2 JSON envelope with --output json, its follow-ups spelled by commandLine.
func New(streams *output.Streams, env uicli.Env, open func(ctx context.Context, dir string) (runup.UseCase, error), commandLine output.CommandLine) *cobra.Command {
	c := &upCommand{io: uicli.Run{Command: runup.Name, Streams: streams, Env: env, CommandLine: commandLine, Flags: upFlags}, form: newForm(open)}
	cmd := &cobra.Command{Use: "up [worktree...]", Short: upShort, Long: upLong, Example: upExample, Args: cobra.ArbitraryArgs, RunE: c.run}
	c.form.Bind(cmd, &c.io)

	return cmd
}

func (c *upCommand) run(cmd *cobra.Command, args []string) error {
	return c.form.Launch(cmd, args, c.form.Conclude)
}

// Line spells a follow-up that names run up, from run up's schema, as wtm-next runs it; none when its answers are not run up's.
func Line(followUp domain.FollowUp) []string {
	return newForm(nil).Spell(uicli.Program+" "+Use, followUp)
}
