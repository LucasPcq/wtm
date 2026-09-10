package wt

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	syncflow "github.com/LucasPcq/wtm/internal/flow/sync"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/detect"
)

// newSyncCmd creates the wtm sync subcommand.
func newSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdSync + " [branch...]",
		Short: "Rebase selected worktrees onto their parent, in cascade",
		Long: "Rebase one or more managed worktrees onto their parent. Pass branch names to target\n" +
			"specific worktrees, --all to sync every worktree, or no arguments to pick interactively.\n" +
			"The base branch is fetched and fast-forwarded first, then each selected worktree is\n" +
			"rebased onto its parent in topological order (parents before children). The cascade is\n" +
			"local; on a conflict the branch is left clean (rebase aborted) and its selected\n" +
			"descendants are skipped. Pass --keep-conflict to leave a conflicting rebase in progress\n" +
			"in its worktree for manual resolution instead of aborting. A parent no step covers —\n" +
			"a branch with no worktree, or one left out of the selection — is never refreshed by the\n" +
			"cascade; when it is behind its remote you are offered to fast-forward it first\n" +
			"(--ff-parents / --no-ff-parents). After a successful cascade, optionally force-push\n" +
			"(with lease) the rebased branches.",
		Args: cobra.ArbitraryArgs,
		RunE: runSync,
	}

	cmd.Flags().Bool(domain.FlagAll, false, "Sync every managed worktree")
	cmd.Flags().Bool(domain.FlagDryRun, false, "Preview the cascade without rebasing or pushing")
	cmd.Flags().BoolP(domain.FlagYes, "y", false, "Skip all prompts; resolve every decision from flags and safe defaults (requires branch args or --all; use --push to push)")
	cmd.Flags().Bool(domain.FlagPush, false, "Force-push (with lease) rebased branches without prompting")
	cmd.Flags().Bool(domain.FlagNoPush, false, "Rebase locally only; never push")
	cmd.Flags().Bool(domain.FlagKeepConflict, false, "Leave a conflicting rebase in progress in its worktree instead of aborting")
	cmd.Flags().Bool(domain.FlagFFParents, false, "Fast-forward the parents the cascade does not cover (no worktree, or left out of the selection) before rebasing onto them; no-op with --"+domain.FlagDryRun)
	cmd.Flags().Bool(domain.FlagNoFFParents, false, "Never fast-forward those parents; rebase onto them as they are")
	cmd.Flags().String(domain.FlagBase, "", "Base branch to sync from (defaults to config or detected base)")
	shared.AddOutputFlag(cmd)

	return cmd
}

func runSync(cmd *cobra.Command, args []string) error {
	all, _ := cmd.Flags().GetBool(domain.FlagAll)
	dryRun, _ := cmd.Flags().GetBool(domain.FlagDryRun)
	yes, _ := cmd.Flags().GetBool(domain.FlagYes)
	push, _ := cmd.Flags().GetBool(domain.FlagPush)
	noPush, _ := cmd.Flags().GetBool(domain.FlagNoPush)
	keepConflict, _ := cmd.Flags().GetBool(domain.FlagKeepConflict)
	ffParents, _ := cmd.Flags().GetBool(domain.FlagFFParents)
	noFFParents, _ := cmd.Flags().GetBool(domain.FlagNoFFParents)
	baseOverride, _ := cmd.Flags().GetString(domain.FlagBase)
	format, _ := cmd.Flags().GetString(domain.FlagOutput)

	if push && noPush {
		return fmt.Errorf("--%s and --%s are mutually exclusive", domain.FlagPush, domain.FlagNoPush)
	}
	if ffParents && noFFParents {
		return fmt.Errorf("--%s and --%s are mutually exclusive", domain.FlagFFParents, domain.FlagNoFFParents)
	}
	if all && len(args) > 0 {
		return fmt.Errorf("--%s cannot be combined with branch arguments", domain.FlagAll)
	}

	if format == domain.OutputJSON && !yes && !dryRun {
		return fmt.Errorf("--output json requires --%s or --%s (prompts cannot run in JSON mode)", domain.FlagYes, domain.FlagDryRun)
	}

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	config, err := shared.LoadConfig(cmd, dir)
	if err != nil {
		return err
	}

	// --dry-run keeps the picker: it still selects what to preview, it just never
	// confirms — which is why it is not folded into interactive, only into the
	// refusal below.
	interactive := rules.IsHumanFormat(format) && term.IsTerminal(int(os.Stdin.Fd())) && !yes
	if !interactive && !yes && !dryRun {
		return errors.New(domain.SyncNeedsTerminal)
	}

	_, err = syncflow.Run(syncflow.Params{
		Context: shared.FlowContext(config),
		Request: syncflow.Request{
			Branches:     args,
			All:          all,
			KeepConflict: keepConflict,
			FFParents:    ffParents,
			NoFFParents:  noFFParents,
			Push:         push,
			NoPush:       noPush,
			DryRun:       dryRun,
			BaseBranch:   resolveBase(baseOverride, config),
		},
		// The picker may be reached through the shell wrapper, which consumes stdout.
		Prompter:  shared.FlowPrompter(shared.FlowPrompterParams{Interactive: interactive, Stderr: true}),
		Presenter: syncPresenter{CLIPresenter: shared.NewPresenter(cmd, format)},
	})
	return err
}

func resolveBase(override string, cfg shared.ConfigResult) string {
	if override != "" {
		return override
	}
	if cfg.Config.Project.Worktrees.BaseBranch != "" {
		return cfg.Config.Project.Worktrees.BaseBranch
	}
	return detect.BaseBranch(cfg.ProjectDir)
}
