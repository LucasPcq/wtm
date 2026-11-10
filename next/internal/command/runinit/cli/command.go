// Package cli is run init's command line: its flags, its form, its output and its JSON, over run init's use case.
package cli

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/command/runinit"
	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/cobra"
)

// Use is how the command line spells run init: a subcommand of run.
const Use = "run init"

const (
	initShort = "Set up what runs in each worktree"
	initLong  = "Set up the run module of the repository you stand in, from what it holds: the services of its\n" +
		"docker-compose.yml, the dev and start scripts of its apps and the other scripts of its packages, and\n" +
		"its .env files. Six parts, in order: what runs; shared or per worktree, and what each worktree gets\n" +
		"of a shared service; each job's port; the jobs answering under their own name; the .env keys that\n" +
		"follow a job; the profiles run up starts. Then run.toml, docker-compose.yml's rewrite and\n" +
		"config.toml's .env files are written at once, or none.\n" +
		"\n" +
		"A compose service's literal host port is rewritten to read the variable its port is declared under,\n" +
		"the literal kept as its default (\"${POSTGRES_PORT:-5432}:5432\"), so docker compose up keeps working\n" +
		"on its own; a container's name is fronted with the compose project. --patch-compose=false leaves\n" +
		"the file as it is.\n" +
		"\n" +
		"Run again, it sets the module up again from run.toml: every part filled from it, a job, a link or a\n" +
		"profile left out staying out, nothing found since added unattended; a job run.toml declares that\n" +
		"run init does not propose (run job add's) is kept as it is."
	initExample = `  # The wizard
  wtm run init

  # Unattended, from what wtm finds
  wtm run init --yes

  # Leave docker-compose.yml alone, start the apps through turbo
  wtm run init --yes --patch-compose=false --runner through

  # Fix admin's port and copy web's .env.local into new worktrees
  wtm run init --yes --command 'admin=pnpm exec next dev -p ${ADMIN_PORT}' --env-copy apps/web/.env.local

  # See the plan first, then apply exactly that plan
  wtm run init --yes --dry-run
  wtm run init --expect-fingerprint <hash>

  # For a script or an agent, with the v2 JSON envelope
  wtm run init --yes --output json`

	yesUsage = "Run unattended: no question, no confirmation (each part as given, else as run.toml holds it, else as found)"
)

type runInitCommand struct {
	io   uicli.Run
	form *runInitForm
}

// New returns the run init command: its flags become a runinit.Request, answered in the wizard when the user can see and answer it, else run unattended; it draws the outcome on streams, or the v2 JSON envelope with --output json, its follow-ups spelled by commandLine.
func New(streams *output.Streams, env uicli.Env, open func(ctx context.Context, dir string) (runinit.UseCase, error), commandLine output.CommandLine) *cobra.Command {
	c := &runInitCommand{io: uicli.Run{Command: runinit.Name, Streams: streams, Env: env, CommandLine: commandLine, Flags: runInitFlags}, form: newForm(open)}
	cmd := &cobra.Command{Use: "init", Short: initShort, Long: initLong, Example: initExample, Args: cobra.NoArgs, RunE: c.run}
	c.form.Bind(cmd, &c.io)

	return cmd
}

func (c *runInitCommand) run(cmd *cobra.Command, args []string) error {
	return c.form.Launch(cmd, args, c.form.Conclude)
}

// Line spells a follow-up that names run init, from run init's schema, as wtm-next runs it; none when its answers are not run init's.
func Line(followUp domain.FollowUp) []string {
	return newForm(nil).Spell(uicli.Program+" "+Use, followUp)
}
