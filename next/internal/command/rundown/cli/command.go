// Package cli is run down's command line: its flags, its form, its output and its JSON, over run down's use case.
package cli

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/command/rundown"
	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/cobra"
)

// Use is how the command line spells run down after the program's name.
const Use = "run down"

const (
	downShort = "Stop a worktree's jobs"
	downLong  = "Stop every job the run daemon runs in each worktree named, or in the one you stand in.\n" +
		"With --profile, only that profile's jobs stop; with --all, every worktree of this repository's,\n" +
		"never another repository's. Several worktrees stop at once.\n" +
		"\n" +
		"A shared service a worktree holds is released: it runs on for the worktrees still holding it, and\n" +
		"stops with its last hold. A stack stops by its stop command. A job that crashed is cleared.\n" +
		"A worktree removed while its jobs ran is named by its path."
	downExample = `  # The jobs of the worktree you stand in
  wtm run down

  # Two worktrees, one profile's jobs only
  wtm run down feat/login fix/typo --profile backend

  # Every worktree of this repository, for a script or an agent
  wtm run down --all --yes --output json`

	yesUsage = "Run unattended: no question, no confirmation (the worktree you stand in unless given)"
)

type downCommand struct {
	io   uicli.Run
	form *downForm
}

// New returns run down's command, a subcommand of run: its flags and arguments become a rundown.Request, answered in the wizard when the user can see and answer it, else run unattended; it draws the outcome on streams, or the v2 JSON envelope with --output json, its follow-ups spelled by commandLine.
func New(streams *output.Streams, env uicli.Env, open func(ctx context.Context, dir string) (rundown.UseCase, error), commandLine output.CommandLine) *cobra.Command {
	c := &downCommand{io: uicli.Run{Command: rundown.Name, Streams: streams, Env: env, CommandLine: commandLine, Flags: downFlags}, form: newForm(open)}
	cmd := &cobra.Command{Use: "down [worktree...]", Short: downShort, Long: downLong, Example: downExample, Args: cobra.ArbitraryArgs, RunE: c.run}
	c.form.Bind(cmd, &c.io)

	return cmd
}

func (c *downCommand) run(cmd *cobra.Command, args []string) error {
	return c.form.Launch(cmd, args, c.form.Conclude)
}

// Line spells a follow-up that names run down, from run down's schema, as wtm-next runs it; none when its answers are not run down's.
func Line(followUp domain.FollowUp) []string {
	return newForm(nil).Spell(uicli.Program+" "+Use, followUp)
}
