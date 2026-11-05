// Package cli is run start's command line: its flags, its form, its output and its JSON, over run start's use case.
package cli

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/command/runstart"
	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
	"github.com/LucasPcq/wtm/next/internal/ui/runview"

	"github.com/spf13/cobra"
)

// Use is how the command line spells run start after the program's name.
const Use = "run start"

const (
	startShort = "Start one job, or start it again"
	startLong  = "Start one job of run.toml in the worktree named, or in the one you stand in. A service up\n" +
		"there is left as it is, unless --restart stops it and starts it again, its log continuing\n" +
		"behind a [wtm] started line. A shared service runs once, in the main checkout, and the\n" +
		"worktree holds it; it is never restarted for one worktree (run stop, then run start).\n" +
		"\n" +
		"A task runs to its end; a service is launched and its ports checked (--no-probe skips it).\n" +
		"When other worktrees run jobs, it asks whether to stop them first, unless --exclusive,\n" +
		"--parallel or run.toml's concurrency answers, never for a task; --yes keeps them. A job\n" +
		"whose touches reach data the worktree does not own is refused unless --force."
	startExample = `  # The api of the worktree you stand in
  wtm run start --job api

  # Restart it in another worktree
  wtm run start feat/login --job api --restart

  # For a script or an agent, with the v2 JSON envelope
  wtm run start feat/login --job migrate --yes --output json`

	yesUsage = "Run unattended: no question, no confirmation (the worktree you stand in unless given; --job is required; the other worktrees' jobs keep running unless --exclusive)"
)

type startCommand struct {
	io   uicli.Run
	form *startForm
	view runview.View
}

// New returns run start's command, a subcommand of run: its flags and argument become a runstart.Request, answered in the wizard when the user can see and answer it, else run unattended; it draws the outcome on streams, or the v2 JSON envelope with --output json, its follow-ups spelled by commandLine; at a terminal, the service it left up is shown live in view first.
func New(streams *output.Streams, env uicli.Env, open func(ctx context.Context, dir string) (runstart.UseCase, error), commandLine output.CommandLine, view runview.View) *cobra.Command {
	c := &startCommand{io: uicli.Run{Command: runstart.Name, Streams: streams, Env: env, CommandLine: commandLine, Flags: startFlags}, form: newForm(open), view: view}
	cmd := &cobra.Command{Use: "start [worktree]", Short: startShort, Long: startLong, Example: startExample, Args: cobra.MaximumNArgs(1), RunE: c.run}
	c.form.Bind(cmd, &c.io)

	return cmd
}

func (c *startCommand) run(cmd *cobra.Command, args []string) error {
	return c.form.Launch(cmd, args, c.conclude)
}

// Line spells a follow-up that names run start, from run start's schema, as wtm-next runs it; none when its answers are not run start's.
func Line(followUp domain.FollowUp) []string {
	return newForm(nil).Spell(uicli.Program+" "+Use, followUp)
}
