// Package cli is sync's command line: its flags, its form, its output and its JSON, over sync's use case.
package cli

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/command/sync"
	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
	"github.com/spf13/cobra"
)

const (
	syncShort = "Rebase worktrees onto their parent, in cascade"
	syncLong  = "Rebase each worktree onto its parent, the branch it was created from, parents before children:\n" +
		"origin is fetched once, the branches rebased onto but not rebased (the base branch among them)\n" +
		"are fast-forwarded to origin first, then each worktree is rebased in turn, after catching up\n" +
		"with its own branch on origin. Without arguments, the wizard asks which worktrees to sync, those\n" +
		"that need it checked; unattended, those that need it are synced. A worktree is named by its\n" +
		"branch or its path; --all asks for every worktree.\n" +
		"\n" +
		"By default the worktrees under each one named are synced too, and the plan names each of them;\n" +
		"--descendants=false syncs the named alone. --ff-parents=false rebases onto the parents as they\n" +
		"stand.\n" +
		"\n" +
		"A rebase that stops on a conflict is aborted, the worktree left as it was, and the worktrees\n" +
		"under it are skipped; --keep-conflict leaves it in progress to resolve, then sync it again to\n" +
		"sync what is left. --push force-pushes each branch synced, leasing on origin's branch as fetched.\n" +
		"\n" +
		"A worktree in the middle of a rebase, a bisect or a merge, whose directory is gone, whose parent\n" +
		"is gone or leads back to it, or on whose branch origin has commits it never held is left as it\n" +
		"is, with what moves it on.\n" +
		"\n" +
		"A worktree with uncommitted changes is unsafe: named, it refuses the whole run before anything\n" +
		"moves, unless --force, which stashes its changes around the rebase; not named, it is kept as it\n" +
		"is, with the line that forces it. --force lifts that refusal only: it still asks to confirm,\n" +
		"unless --yes."
	syncExample = `  # Pick the worktrees to sync
  wtm sync

  # A worktree and every worktree under it
  wtm sync feat/login

  # That worktree alone
  wtm sync feat/login --descendants=false

  # Every worktree, then force-push them, no prompts
  wtm sync --all --push --yes

  # See the plan first, then apply exactly that plan
  wtm sync feat/login --dry-run
  wtm sync feat/login --expect-fingerprint <hash>

  # For a script or an agent, with the v2 JSON envelope
  wtm sync --all --yes --output json`
	yesUsage = "Run unattended: no question, no confirmation (without worktrees, those that need a sync; the worktrees under those named come too; nothing is pushed unless --push; a worktree named with uncommitted changes still refuses the run unless --force)"
)

type syncCommand struct {
	io   uicli.Run
	form *syncForm
}

// New returns the sync command: its flags become a sync.Request, answered in the wizard when the user can see and answer it, else run unattended; it draws the outcome on streams, or the v2 JSON envelope with --output json, its follow-ups spelled by commandLine.
func New(streams *output.Streams, env uicli.Env, open func(ctx context.Context, dir string) (sync.UseCase, error), commandLine output.CommandLine) *cobra.Command {
	c := &syncCommand{io: uicli.Run{Command: sync.Name, Streams: streams, Env: env, CommandLine: commandLine, Flags: syncFlags}, form: newForm(open)}
	cmd := &cobra.Command{Use: sync.Name + " [worktree...]", Short: syncShort, Long: syncLong, Example: syncExample, Args: cobra.ArbitraryArgs, RunE: c.run}
	c.form.Bind(cmd, &c.io)
	return cmd
}

func (c *syncCommand) run(cmd *cobra.Command, args []string) error {
	return c.form.Launch(cmd, args, c.form.Conclude)
}

// Line spells a follow-up that names sync, from sync's schema, as wtm-next runs it; none when its answers are not sync's.
func Line(followUp domain.FollowUp) []string {
	return newForm(nil).Spell(uicli.Program+" "+sync.Name, followUp)
}
