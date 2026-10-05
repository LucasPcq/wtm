package wt

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	relocateflow "github.com/LucasPcq/wtm/internal/flow/relocate"
	"github.com/LucasPcq/wtm/internal/rules"
)

func newRelocateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdRelocate,
		Short: "Move worktrees to align with base_path and adopt external ones",
		Long: "Reconcile every worktree with the configured base_path. Worktrees not under it are\n" +
			"moved (git worktree move) and worktrees created outside wtm are adopted (their parent\n" +
			"recorded so `wtm sync` works). Pass --to to change base_path and move existing worktrees\n" +
			"to the new location. Dirty or locked worktrees are skipped unless --force; an occupied\n" +
			"target path is never overwritten, and a worktree whose jobs are running is never moved\n" +
			"(stop them with `wtm run down <branch>` first). Adoption keeps what the worktree's\n" +
			"meta.json already records (isolation, namespaces, ordinal).",
		Example: `  # Show the plan first
  wtm relocate --dry-run

  wtm relocate

  # Move every worktree under a new directory
  wtm relocate --to ../acme.trees --yes`,
		Args: cobra.NoArgs,
		RunE: runRelocate,
	}

	cmd.Flags().String(domain.FlagTo, "", "New base_path (relative to repo root); also moves existing worktrees there")
	cmd.Flags().Bool(domain.FlagDryRun, false, "Preview the plan without moving or adopting anything")
	cmd.Flags().Bool(domain.FlagForce, false, "Lift safety refusals (dirty/locked): move those worktrees too; still asks to confirm unless --yes")
	cmd.Flags().BoolP(domain.FlagYes, "y", false, "Skip all prompts; resolve every decision from flags and safe defaults (parents default to the base branch)")
	shared.AddOutputFlag(cmd)

	return cmd
}

func runRelocate(cmd *cobra.Command, _ []string) error {
	to, _ := cmd.Flags().GetString(domain.FlagTo)
	dryRun, _ := cmd.Flags().GetBool(domain.FlagDryRun)
	force, _ := cmd.Flags().GetBool(domain.FlagForce)
	yes, _ := cmd.Flags().GetBool(domain.FlagYes)
	format, _ := cmd.Flags().GetString(domain.FlagOutput)

	if err := rules.ValidateRelocateTarget(to); err != nil {
		return err
	}

	if format == domain.OutputJSON && !yes && !dryRun {
		return fmt.Errorf("--output json requires --%s or --%s (the confirmation cannot run in JSON mode)", domain.FlagYes, domain.FlagDryRun)
	}

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	cfg, err := shared.LoadConfig(cmd, dir)
	if err != nil {
		return err
	}

	// A run that can neither confirm nor was told to proceed refuses rather than
	// moving worktrees unasked. --dry-run writes nothing, so it needs neither.
	canPrompt := rules.IsHumanFormat(format) && term.IsTerminal(int(os.Stdin.Fd()))
	if !canPrompt && !yes && !dryRun {
		return fmt.Errorf("relocate needs a terminal to confirm; re-run with --%s to proceed unattended", domain.FlagYes)
	}

	_, err = relocateflow.Run(cmd.Context(), relocateflow.Params{
		Context: shared.FlowContext(cfg),
		Request: relocateflow.Request{
			To:         to,
			Force:      force,
			DryRun:     dryRun,
			BaseBranch: resolveBase(cmd.Context(), "", cfg),
		},
		Prompter:  shared.FlowPrompter(cmd.Context(), shared.FlowPrompterParams{Interactive: canPrompt && !yes && !dryRun, Stderr: true}),
		Presenter: relocatePresenter{CLIPresenter: shared.NewPresenter(cmd, format)},
	})
	return err
}
