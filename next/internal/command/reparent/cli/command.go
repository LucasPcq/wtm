// Package cli is reparent's command line: its flags, its form, its output and its JSON, over reparent's use case.
package cli

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/command/reparent"
	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/cobra"
)

const (
	reparentShort = "Change the parent worktrees are rebased onto"
	reparentLong  = "Change the parent branch worktrees are rebased onto: the next `wtm sync` rebases them onto it.\n" +
		"Only the meta record changes: no branch, commit nor file moves. Their children follow them.\n" +
		"Without arguments, the worktree you stand in moves; the wizard asks which worktrees to move\n" +
		"and onto which branch, a local branch or origin/<branch>.\n" +
		"\n" +
		"A worktree cannot sit under itself: a parent that is one of the worktrees moved, or sits under\n" +
		"one of them, is refused. Never the main worktree, the base branch's, one on no branch or one\n" +
		"wtm did not create. A worktree already on the parent is left as it is."
	reparentExample = `  # Move the worktree you stand in
  wtm reparent --to main

  # Several at once, no prompts
  wtm reparent feat/b feat/c --to feat/a --yes

  # See the plan first, then apply exactly that plan
  wtm reparent feat/b --to main --dry-run
  wtm reparent feat/b --to main --expect-fingerprint <hash>

  # For a script or an agent, with the v2 JSON envelope
  wtm reparent feat/b --to main --yes --output json`

	yesUsage = "Run unattended: no question, no confirmation (--to required; the worktree you stand in unless worktrees are named)"
)

type reparentCommand struct {
	io   uicli.Run
	form *reparentForm
}

// New returns the reparent command: its flags become a reparent.Request, answered in the wizard when the user can see and answer it, else run unattended; it draws the outcome on streams, or the v2 JSON envelope with --output json, its follow-ups spelled by commandLine.
func New(streams *output.Streams, env uicli.Env, open func(ctx context.Context, dir string) (reparent.UseCase, error), commandLine output.CommandLine) *cobra.Command {
	c := &reparentCommand{io: uicli.Run{Command: reparent.Name, Streams: streams, Env: env, CommandLine: commandLine, Flags: reparentFlags}, form: newForm(open)}
	cmd := &cobra.Command{Use: reparent.Name + " [worktree...]", Short: reparentShort, Long: reparentLong, Example: reparentExample, Args: cobra.ArbitraryArgs, RunE: c.run}
	c.form.Bind(cmd, &c.io)

	return cmd
}

func (c *reparentCommand) run(cmd *cobra.Command, args []string) error {
	return c.form.Launch(cmd, args, c.form.Conclude)
}

// Line spells a follow-up that names reparent, from reparent's schema, as wtm-next runs it; none when its answers are not reparent's.
func Line(followUp domain.FollowUp) []string {
	return newForm(nil).Spell(uicli.Program+" "+reparent.Name, followUp)
}
