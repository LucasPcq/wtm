// Package cli is init's command line: its flags, its form, its output and its JSON, over init's use case.
package cli

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/command/initialize"
	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/cobra"
)

const (
	initShort = "Set wtm up in this repository"
	initLong  = "Set wtm up in the repository you stand in: where its worktrees go, the branch they start from,\n" +
		"the .env files each one gets and the commands run when one is created or removed. The settings\n" +
		"stay on this machine, in the repository's git directory, out of git.\n" +
		"\n" +
		"A first init fills each setting from what it finds: origin's default branch, the .env files and\n" +
		"their templates, the install command of the package manager, a docker compose file to stop.\n" +
		"Run again, it sets the repository up again from its settings: each one as it is, every .env file\n" +
		"it finds listed, only those set up picked. Unattended, a flag changes its setting alone.\n" +
		"\n" +
		"A hook is a command, or JSON for one that runs in a folder or goes on after a failure:\n" +
		"  --on-create '{\"cmd\": \"pnpm db:generate\", \"cwd\": \"packages/db\"}'\n" +
		"What runs in each worktree is set up by `wtm run init`."
	initExample = `  # The wizard
  wtm init

  # Unattended, from what wtm finds
  wtm init --yes

  # No .env files, two hooks on create
  wtm init --yes --env-strategy none --on-create "pnpm install" --on-create "make setup"

  # Change one setting of a repository set up already
  wtm init --yes --on-clean "docker compose down"

  # See the plan first, then apply exactly that plan
  wtm init --yes --dry-run
  wtm init --expect-fingerprint <hash>

  # For a script or an agent, with the v2 JSON envelope
  wtm init --yes --output json`

	yesUsage = "Run unattended: no question, no confirmation (each setting as given, else as set up, else as found)"
)

type initCommand struct {
	io   uicli.Run
	form *initForm
}

// New returns the init command: its flags become an initialize.Request, answered in the wizard when the user can see and answer it, else run unattended; it draws the outcome on streams, or the v2 JSON envelope with --output json, its follow-ups spelled by commandLine.
func New(streams *output.Streams, env uicli.Env, open func(ctx context.Context, dir string) (initialize.UseCase, error), commandLine output.CommandLine) *cobra.Command {
	c := &initCommand{io: uicli.Run{Command: initialize.Name, Streams: streams, Env: env, CommandLine: commandLine, Flags: initFlags}, form: newForm(open)}
	cmd := &cobra.Command{Use: initialize.Name, Short: initShort, Long: initLong, Example: initExample, Args: cobra.NoArgs, RunE: c.run}
	c.form.Bind(cmd, &c.io)

	return cmd
}

func (c *initCommand) run(cmd *cobra.Command, args []string) error {
	return c.form.Launch(cmd, args, c.form.Conclude)
}

// Line spells a follow-up that names init, from init's schema, as wtm-next runs it; none when its answers are not init's.
func Line(followUp domain.FollowUp) []string {
	return newForm(nil).Spell(uicli.Program+" "+initialize.Name, followUp)
}
