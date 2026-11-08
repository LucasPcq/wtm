// Package cli is daemon stop-jobs' command line: its flag, its form, its output and its JSON, over stop-jobs' use case.
package cli

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/command/daemonstopjobs"
	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/cobra"
)

// Use is how the command line spells daemon stop-jobs after the program's name.
const Use = "daemon stop-jobs"

const (
	stopShort = "Stop every job wtm's daemon runs, in every repository"
	stopLong  = "Stop every job wtm's daemon runs, in every repository of this machine, or with\n" +
		"--repo in one: each worktree's services, its stacks by their stop command, its holds\n" +
		"on shared services, which stop with their last hold; a crashed job is cleared. The\n" +
		"worktrees stop at once. The daemon and its proxy stay up, and nothing starts again\n" +
		"on its own: `wtm run up` in a worktree starts its jobs again.\n\n" +
		"It asks first; --yes stops without asking."
	stopExample = `  wtm daemon stop-jobs

  # One repository's, for a script or an agent
  wtm daemon stop-jobs --repo ~/src/acme --yes --output json`

	yesUsage = "Stop without asking"
)

type stopJobsCommand struct {
	io   uicli.Run
	form *stopJobsForm
}

// New returns daemon stop-jobs' command, a subcommand of daemon: its flag becomes a daemonstopjobs.Request, its plan confirmed when the user can see and answer it, else run unattended; it draws the outcome on streams, or the v2 JSON envelope with --output json, its follow-ups spelled by commandLine.
func New(streams *output.Streams, env uicli.Env, open func(ctx context.Context, dir string) (daemonstopjobs.UseCase, error), commandLine output.CommandLine) *cobra.Command {
	c := &stopJobsCommand{io: uicli.Run{Command: daemonstopjobs.Name, Streams: streams, Env: env, CommandLine: commandLine, Flags: stopJobsFlags}, form: newForm(open)}
	cmd := &cobra.Command{Use: "stop-jobs", Short: stopShort, Long: stopLong, Example: stopExample, Args: cobra.NoArgs, RunE: c.run}
	c.form.Bind(cmd, &c.io)

	return cmd
}

func (c *stopJobsCommand) run(cmd *cobra.Command, args []string) error {
	return c.form.Launch(cmd, args, c.form.Conclude)
}

// Line spells a follow-up that names daemon stop-jobs, as wtm-next runs it.
func Line(followUp domain.FollowUp) []string {
	return newForm(nil).Spell(uicli.Program+" "+Use, followUp)
}
