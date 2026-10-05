package wt

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	pruneflow "github.com/LucasPcq/wtm/internal/flow/prune"
	"github.com/LucasPcq/wtm/internal/rules"
)

// newPruneCmd creates the wtm prune subcommand.
func newPruneCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdPrune,
		Short: "Remove finished worktrees (merged, closed PR or gone) in one pass",
		Long: "Batch-remove worktrees whose work is done, reparenting any surviving children onto\n" +
			"their nearest surviving ancestor (like `clean --reparent-children`). Whether work is \"done\" is read\n" +
			"from GitHub via the `gh` CLI — never guessed from local commits — so squash- and\n" +
			"rebase-merges are detected correctly. By default prune considers every finished\n" +
			"worktree: merged PR, closed PR, or upstream branch gone. The reason flags restrict to\n" +
			"specific categories — --merged (PR merged), --closed (PR closed unmerged), --gone\n" +
			"(remote branch deleted).\n" +
			"\n" +
			"--merged and --closed require the GitHub CLI (`gh`) to be installed and authenticated;\n" +
			"without it they match nothing and prune prints a notice — only --gone still applies.\n" +
			"gone-detection first fetches the worktrees' branches from origin, dropping the\n" +
			"remote-tracking refs of those deleted there (pass --no-fetch to skip). Only branches\n" +
			"with a worktree are read, on git and on GitHub: the remote-tracking refs of the\n" +
			"other branches are left as they are.\n" +
			"\n" +
			"On a TTY, matches are shown for review (unsafe ones unchecked), then a prune\n" +
			"confirmation, then — like clean — a dedicated confirmation to reparent surviving\n" +
			"children onto their nearest surviving ancestor (or leave them orphaned). The main checkout and base\n" +
			"branch are always protected; the current worktree is removed and the shell\n" +
			"redirected to the base repo. Like clean, worktrees that are locked, dirty, have\n" +
			"unpushed commits, or have an open PR are unsafe and need --force. Use --yes to skip the\n" +
			"prompts (required with --output json); non-interactively, children are left orphaned\n" +
			"unless --reparent-children is passed. --dry-run previews without changing anything.\n" +
			"\n" +
			"Like clean, prune gives back the data the removed worktrees carved out of shared\n" +
			"services (--keep-data withholds it); when such a service is down, the form asks whether\n" +
			"to start it and drop the data now, or keep it until the service next starts. --yes keeps\n" +
			"it; --drop-data drops it, starting the services that are down.\n" +
			"\n" +
			"Each worktree goes through clean's whole sequence — jobs stopped, hooks, removal, then its\n" +
			"data — before the next one starts. The first that fails stops the prune: the ones before\n" +
			"it are gone with their data, it and the ones after keep theirs, and the report (and the\n" +
			"`failed` field of --output json) names where it stopped.",
		Example: `  # Review every finished worktree, then confirm
  wtm prune

  # Only show what would go
  wtm prune --dry-run

  # Every worktree whose PR was merged, no prompts
  wtm prune --merged --yes --reparent-children`,
		Args: cobra.NoArgs,
		RunE: runPrune,
	}

	cmd.Flags().Bool(domain.FlagMerged, false, "Restrict to worktrees whose PR was merged on GitHub (needs gh)")
	cmd.Flags().Bool(domain.FlagClosed, false, "Restrict to worktrees whose PR was closed without merging (needs gh)")
	cmd.Flags().Bool(domain.FlagGone, false, "Restrict to worktrees whose upstream branch was deleted on the remote")
	cmd.Flags().Bool(domain.FlagNoFetch, false, "Skip the fetch of the worktrees' branches that gone-detection performs; use already-fetched state")
	cmd.Flags().Bool(domain.FlagForce, false, "Lift safety refusals (locked/dirty/unpushed/open-PR): also remove unsafe worktrees; still asks to confirm unless --yes")
	cmd.Flags().Bool(domain.FlagReparentChildren, false, domain.FlagReparentChildrenDesc)
	cmd.Flags().BoolP(domain.FlagYes, "y", false, "Skip all prompts; keep every match without the selection picker (use --force for unsafe worktrees)")
	cmd.Flags().Bool(domain.FlagDryRun, false, "Preview what would be pruned without removing anything")
	cmd.Flags().Bool(domain.FlagKeepData, false, domain.FlagKeepDataDesc)
	cmd.Flags().Bool(domain.FlagDropData, false, domain.FlagDropDataDesc)
	cmd.MarkFlagsMutuallyExclusive(domain.FlagKeepData, domain.FlagDropData)
	shared.AddOutputFlag(cmd)
	shared.RequireYesInJSON(cmd, domain.FlagDryRun)

	return cmd
}

func runPrune(cmd *cobra.Command, _ []string) error {
	merged, _ := cmd.Flags().GetBool(domain.FlagMerged)
	closed, _ := cmd.Flags().GetBool(domain.FlagClosed)
	gone, _ := cmd.Flags().GetBool(domain.FlagGone)
	noFetch, _ := cmd.Flags().GetBool(domain.FlagNoFetch)
	force, _ := cmd.Flags().GetBool(domain.FlagForce)
	reparentChildren, _ := cmd.Flags().GetBool(domain.FlagReparentChildren)
	yes, _ := cmd.Flags().GetBool(domain.FlagYes)
	dryRun, _ := cmd.Flags().GetBool(domain.FlagDryRun)
	keepData, _ := cmd.Flags().GetBool(domain.FlagKeepData)
	dropData, _ := cmd.Flags().GetBool(domain.FlagDropData)
	format, _ := cmd.Flags().GetString(domain.FlagOutput)

	// Default is broad: with no reason flag, consider every finished worktree
	// (merged, closed-PR, or gone). Reason flags narrow to specific categories.
	if !merged && !closed && !gone {
		merged, closed, gone = true, true, true
	}

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	config, err := shared.LoadConfig(cmd, dir)
	if err != nil {
		return err
	}

	// --dry-run is a bypass of its own: it asks nothing, so it needs neither a
	// terminal nor --yes.
	interactive := rules.IsHumanFormat(format) && term.IsTerminal(int(os.Stdin.Fd())) && !yes && !dryRun
	if !interactive && !yes && !dryRun {
		return errors.New(domain.PruneNeedsTerminal)
	}

	_, err = pruneflow.Run(cmd.Context(), pruneflow.Params{
		Context: shared.FlowContext(config),
		Request: pruneflow.Request{
			Merged:           merged,
			Closed:           closed,
			Gone:             gone,
			NoFetch:          noFetch,
			Force:            force,
			ReparentChildren: reparentChildren,
			DryRun:           dryRun,
			BaseBranch:       resolveBase(cmd.Context(), "", config),
			KeepData:         keepData,
			DropData:         dropData,
		},
		// The picker may be reached through the shell wrapper, which consumes stdout.
		Prompter:  shared.FlowPrompter(cmd.Context(), shared.FlowPrompterParams{Interactive: interactive, Stderr: true}),
		Presenter: prunePresenter{CLIPresenter: shared.NewPresenter(cmd, format)},
	})
	return err
}
