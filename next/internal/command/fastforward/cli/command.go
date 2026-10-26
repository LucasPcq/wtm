// Package cli is fast-forward's command line: its flags, its form, its output and its JSON, over fast-forward's use case.
package cli

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/command/fastforward"
	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/cobra"
)

const (
	fastForwardShort = "Fast-forward worktrees to their origin branch"
	fastForwardLong  = "Bring each worktree's branch up to its branch on origin, never merging: origin is fetched once,\n" +
		"then each worktree behind it moves in turn. A failure does not stop the others.\n" +
		"Without arguments, the wizard asks which worktrees to fast-forward, those behind origin\n" +
		"checked. A worktree is named by its branch or its path.\n" +
		"\n" +
		"A worktree ahead of origin, diverged from it, not on origin, in the middle of a rebase, a\n" +
		"bisect or a merge, or whose directory is gone is left as it is, with what moves it on.\n" +
		"\n" +
		"A worktree behind origin with uncommitted changes is unsafe: named, it refuses the whole run\n" +
		"before anything moves, unless --force; with --all, it is left as it is, with the line that\n" +
		"forces it. --force lifts that refusal only: it still asks to confirm, unless --yes."
	fastForwardExample = `  # Pick the worktrees to fast-forward
  wtm fast-forward

  wtm fast-forward feat/login

  # Every worktree, no prompts
  wtm ff --all --yes

  # See the plan first, then apply exactly that plan
  wtm fast-forward feat/login --dry-run
  wtm fast-forward feat/login --expect-fingerprint <hash>

  # For a script or an agent, with the v2 JSON envelope
  wtm fast-forward --all --yes --output json`

	yesUsage = "Run unattended: no question, no confirmation (worktrees or --all required; a worktree named with uncommitted changes still refuses the run unless --force)"
)

type fastForwardCommand struct {
	io   uicli.Run
	form *fastForwardForm
}

// New returns the fast-forward command: its flags become a fastforward.Request, answered in the wizard when the user can see and answer it, else run unattended; it draws the outcome on streams, or the v2 JSON envelope with --output json, its follow-ups spelled by commandLine.
func New(streams *output.Streams, env uicli.Env, open func(ctx context.Context, dir string) (fastforward.UseCase, error), commandLine output.CommandLine) *cobra.Command {
	c := &fastForwardCommand{io: uicli.Run{Command: fastforward.Name, Streams: streams, Env: env, CommandLine: commandLine, Flags: fastForwardFlags}, form: newForm(open)}
	cmd := &cobra.Command{Use: fastforward.Name + " [worktree...]", Aliases: []string{"ff"}, Short: fastForwardShort, Long: fastForwardLong, Example: fastForwardExample, Args: cobra.ArbitraryArgs, RunE: c.run}
	c.form.Bind(cmd, &c.io)

	return cmd
}

func (c *fastForwardCommand) run(cmd *cobra.Command, args []string) error {
	return c.form.Launch(cmd, args, c.form.Conclude)
}

// Line spells a follow-up that names fast-forward, from fast-forward's schema, as wtm-next runs it; none when its answers are not fast-forward's.
func Line(followUp domain.FollowUp) []string {
	return newForm(nil).Spell(uicli.Program+" "+fastforward.Name, followUp)
}
