// Package cli is prune's command line: its flags, its form, its output and its JSON, over prune's use case.
package cli

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/command/prune"
	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/cobra"
)

const (
	pruneShort = "Remove the worktrees whose work is done"
	pruneLong  = "Remove the worktrees done with, their local branches and their meta records. The remote branch is never touched.\n" +
		"A worktree is done with when its pull request merged or was closed while its branch stands at\n" +
		"the pull request's head (a commit made since keeps it), or when origin deleted the branch it\n" +
		"tracks. --merged, --closed and --gone pick among these reasons; none given asks for all three.\n" +
		"Origin is fetched first for --gone, unless --no-fetch. Never the main worktree nor the base branch.\n" +
		"\n" +
		"Without arguments, every worktree done with that is safe to remove is pruned; the wizard lists\n" +
		"them all, the safe ones checked. Name worktrees to prune only those.\n" +
		"\n" +
		"A worktree that is locked, has uncommitted changes, holds commits no other branch nor remote\n" +
		"holds, has an open pull request or runs jobs is unsafe. One not named is kept, and named with\n" +
		"the command that prunes it anyway; one named refuses the whole run before anything is removed,\n" +
		"unless --force. --force lifts that refusal only: it still asks to confirm, unless --yes.\n" +
		"\n" +
		"Each worktree is removed in turn, its on_clean hooks first: a failing hook keeps it. A failure\n" +
		"does not stop the others. The children a removal leaves behind stay on their removed parent,\n" +
		"named with the `wtm reparent` that moves them, unless --reparent-children moves them onto the\n" +
		"nearest worktree that stays, else the base branch.\n" +
		"\n" +
		"Until the run module moves to wtm-next, prune neither stops a worktree's jobs nor drops the\n" +
		"data it holds in shared services: jobs up make it unsafe, and the data kept is named."
	pruneExample = `  # Prune every worktree done with that is safe to remove
  wtm prune

  # Only those whose pull request merged; their children move onto the nearest survivor
  wtm prune --merged --reparent-children

  # A worktree kept for being unsafe, pruned anyway
  wtm prune feat/login --force

  # See the plan first, then apply exactly that plan
  wtm prune --dry-run
  wtm prune --expect-fingerprint <hash>

  # For a script or an agent, with the v2 JSON envelope
  wtm prune --output json`

	yesUsage = "Run unattended: no question, no confirmation (every worktree done with that is safe to remove unless worktrees are named; children stay orphaned unless --reparent-children; an unsafe worktree named still refuses the run unless --force)"
)

type pruneCommand struct {
	io   uicli.Run
	form *pruneForm
}

// New returns the prune command: its flags become a prune.Request, answered in the wizard when the user can see and answer it, else run unattended; it draws the outcome on streams, or the v2 JSON envelope with --output json, its follow-ups spelled by commandLine.
func New(streams *output.Streams, env uicli.Env, open func(ctx context.Context, dir string) (prune.UseCase, error), commandLine output.CommandLine) *cobra.Command {
	c := &pruneCommand{io: uicli.Run{Command: prune.Name, Streams: streams, Env: env, CommandLine: commandLine, Flags: pruneFlags}, form: newForm(open)}
	cmd := &cobra.Command{Use: prune.Name + " [worktree...]", Short: pruneShort, Long: pruneLong, Example: pruneExample, Args: cobra.ArbitraryArgs, RunE: c.run}
	c.form.Bind(cmd, &c.io)

	return cmd
}

func (c *pruneCommand) run(cmd *cobra.Command, args []string) error {
	return c.form.Launch(cmd, args, c.form.Conclude)
}

// Line spells a follow-up that names prune, from prune's schema, as wtm-next runs it; none when its answers are not prune's.
func Line(followUp domain.FollowUp) []string {
	return newForm(nil).Spell(uicli.Program+" "+prune.Name, followUp)
}
