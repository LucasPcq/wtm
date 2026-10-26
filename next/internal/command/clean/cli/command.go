// Package cli is clean's command line: its flags, its form, its output and its JSON, over clean's use case.
package cli

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/command/clean"
	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/cobra"
)

const (
	cleanShort = "Remove worktrees and their local branches"
	cleanLong  = "Remove git worktrees, delete their local branches and their meta records. The remote branch is never touched.\n" +
		"Without arguments, the wizard asks which worktrees to remove; several can be checked.\n" +
		"A worktree on no branch is named by its path; a branch no worktree holds is deleted alone.\n" +
		"\n" +
		"A worktree that is locked, has uncommitted changes, holds commits no other branch nor remote\n" +
		"holds, has an open pull request or runs jobs is unsafe: one unsafe worktree refuses the whole\n" +
		"run before anything is removed, unless --force. --force lifts that refusal only: it still\n" +
		"asks to confirm, unless --yes.\n" +
		"\n" +
		"Each worktree is removed in turn, its on_clean hooks first: a failing hook keeps it. A failure\n" +
		"does not stop the others. The children a removal leaves behind stay on their removed parent,\n" +
		"named with the `wtm reparent` that moves them, unless --reparent-children moves them onto the\n" +
		"nearest worktree that stays, else the base branch.\n" +
		"\n" +
		"Until the run module moves to wtm-next, clean neither stops a worktree's jobs nor drops the\n" +
		"data it holds in shared services: jobs up refuse the removal unless --force, and the data\n" +
		"kept is named."
	cleanExample = `  # Pick the worktrees to remove
  wtm clean

  wtm clean feat/login

  # Several at once, no prompts; their children move onto the nearest survivor
  wtm clean feat/login feat/signup --yes --reparent-children

  # See the plan first, then apply exactly that plan
  wtm clean feat/login --dry-run
  wtm clean feat/login --expect-fingerprint <hash>

  # For a script or an agent, with the v2 JSON envelope
  wtm clean feat/login --yes --output json`

	yesUsage = "Run unattended: no question, no confirmation (worktrees required; children stay orphaned unless --reparent-children; an unsafe worktree still refuses the run unless --force)"
)

type cleanCommand struct {
	io   uicli.Run
	form *cleanForm
}

// New returns the clean command: its flags become a clean.Request, answered in the wizard when the user can see and answer it, else run unattended; it draws the outcome on streams, or the v2 JSON envelope with --output json, its follow-ups spelled by commandLine.
func New(streams *output.Streams, env uicli.Env, open func(ctx context.Context, dir string) (clean.UseCase, error), commandLine output.CommandLine) *cobra.Command {
	c := &cleanCommand{io: uicli.Run{Command: clean.Name, Streams: streams, Env: env, CommandLine: commandLine, Flags: cleanFlags}, form: newForm(open)}
	cmd := &cobra.Command{Use: clean.Name + " [worktree...]", Short: cleanShort, Long: cleanLong, Example: cleanExample, Args: cobra.ArbitraryArgs, RunE: c.run}
	c.form.Bind(cmd, &c.io)

	return cmd
}

func (c *cleanCommand) run(cmd *cobra.Command, args []string) error {
	return c.form.Launch(cmd, args, c.form.Conclude)
}

// Line spells a follow-up that names clean, from clean's schema, as wtm-next runs it; none when its answers are not clean's.
func Line(followUp domain.FollowUp) []string {
	return newForm(nil).Spell(uicli.Program+" "+clean.Name, followUp)
}
