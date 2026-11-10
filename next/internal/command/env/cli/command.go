// Package cli is env's command line: its flags, its form, its output and its JSON, over env's use case.
package cli

import (
	"context"

	envcmd "github.com/LucasPcq/wtm/next/internal/command/env"
	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/cobra"
)

const (
	envShort = "Reconcile worktrees' .env files with their sources and what wtm writes"
	envLong  = "Bring each worktree's .env files back in step: a key its source has and a file lacks is added,\n" +
		"a missing file is rebuilt, then the ports and addresses run.toml links and the values wtm owns\n" +
		"are settled for the worktree. Values come from the strategy each worktree was created with\n" +
		"(example, main or parent); --from overrides it for one run.\n" +
		"\n" +
		"A key whose value differs from its source is kept unless --overwrite; a key in no source is\n" +
		"kept unless --prune. Without arguments, the wizard asks which worktrees to reconcile, those out\n" +
		"of step and the one you stand in checked; unattended, the one you stand in alone. A worktree is\n" +
		"named by its branch or its path; --all asks for every worktree.\n" +
		"\n" +
		"Only what wtm writes is shown (the ports and addresses it moves); every other value, secrets\n" +
		"included, is withheld unless --show-values. A job up in a worktree whose .env changed runs on\n" +
		"the old values until restarted: the run says which, with the line that restarts it."
	envExample = `  # Pick the worktrees to reconcile
  wtm env

  wtm env feat/login

  # Also settle the values that differ from their source, and drop the keys no source has
  wtm env feat/login --overwrite --prune --yes

  # See the plan first, then apply exactly that plan
  wtm env --all --dry-run
  wtm env --all --expect-fingerprint <hash>

  # For a script or an agent, with the v2 JSON envelope
  wtm env --all --yes --output json`

	yesUsage = "Run unattended: no question, no confirmation (without worktrees, the one you stand in; a conflict kept unless --overwrite, an orphan unless --prune)"
)

type envCommand struct {
	io   uicli.Run
	form *envForm
}

// New returns the env command: its flags become an env.Request, answered in the wizard when the user can see and answer it, else run unattended; it draws the outcome on streams, or the v2 JSON envelope with --output json, its follow-ups spelled by commandLine.
func New(streams *output.Streams, env uicli.Env, open func(ctx context.Context, dir string) (envcmd.UseCase, error), commandLine output.CommandLine) *cobra.Command {
	c := &envCommand{io: uicli.Run{Command: envcmd.Name, Streams: streams, Env: env, CommandLine: commandLine, Flags: envFlags}, form: newForm(open)}
	cmd := &cobra.Command{Use: envcmd.Name + " [worktree...]", Short: envShort, Long: envLong, Example: envExample, Args: cobra.ArbitraryArgs, RunE: c.run}
	c.form.Bind(cmd, &c.io)

	return cmd
}

func (c *envCommand) run(cmd *cobra.Command, args []string) error {
	return c.form.Launch(cmd, args, c.form.Conclude)
}

// Line spells a follow-up that names env, from env's schema, as wtm-next runs it; none when its answers are not env's.
func Line(followUp domain.FollowUp) []string {
	return newForm(nil).Spell(uicli.Program+" "+envcmd.Name, followUp)
}
