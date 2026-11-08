// Package cli is daemon restart's command line: its form, its output and its JSON, over daemon restart's use case.
package cli

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/command/daemonrestart"
	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/cobra"
)

// Use is how the command line spells daemon restart after the program's name.
const Use = "daemon restart"

const (
	restartShort = "Restart wtm's daemon as this build, its jobs brought back"
	restartLong  = "Restart wtm's daemon, the one process of this machine that runs the jobs of every\n" +
		"repository, as this wtm's build: the way past a daemon of another build, a 0.30\n" +
		"wtm's among them, which refuses new jobs while it holds some.\n\n" +
		"The services it runs itself stop, then start again on the new daemon as they were\n" +
		"started (their profile, worktree, environment and addresses), a shared one with the\n" +
		"worktrees that hold it. Its stacks (a service with a stop command, docker compose\n" +
		"up -d) run on, and the new daemon takes them over. A task running is not run again,\n" +
		"nor a job its index has no record of; the restart names each, and each job that\n" +
		"did not come back.\n\n" +
		"It asks first when a service would stop, if only for a moment; --yes restarts\n" +
		"without asking. With no daemon running, it starts one, and brings back the\n" +
		"services a daemon that died left. A newer wtm's daemon is not replaced without\n" +
		"--force: this build would leave its stacks untracked."
	restartExample = `  wtm daemon restart

  # For a script or an agent
  wtm daemon restart --yes --output json`

	yesUsage = "Restart without asking"
)

type restartCommand struct {
	io   uicli.Run
	form *restartForm
}

// New returns daemon restart's command, a subcommand of daemon: it previews the restart, asks to confirm it when a service would stop and the user can see and answer it, then draws the outcome on streams, or the v2 JSON envelope with --output json, its follow-ups spelled by commandLine.
func New(streams *output.Streams, env uicli.Env, open func(ctx context.Context, dir string) (daemonrestart.UseCase, error), commandLine output.CommandLine) *cobra.Command {
	c := &restartCommand{io: uicli.Run{Command: daemonrestart.Name, Streams: streams, Env: env, CommandLine: commandLine, Flags: restartFlags}, form: newForm(open)}
	cmd := &cobra.Command{Use: "restart", Short: restartShort, Long: restartLong, Example: restartExample, Args: cobra.NoArgs, RunE: c.run}
	c.form.Bind(cmd, &c.io)

	return cmd
}

func (c *restartCommand) run(cmd *cobra.Command, args []string) error {
	return c.form.Launch(cmd, args, c.form.Conclude)
}

// Line spells a follow-up that names daemon restart, as wtm-next runs it.
func Line(followUp domain.FollowUp) []string {
	return newForm(nil).Spell(uicli.Program+" "+Use, followUp)
}
