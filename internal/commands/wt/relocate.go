package wt

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/tui/components"
	relocatetui "github.com/LucasPcq/wtm/internal/tui/relocate"
)

// newRelocateCmd creates the wtm relocate subcommand.
func newRelocateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdRelocate,
		Short: "Move worktrees to align with base_path and adopt external ones",
		Long: "Reconcile every worktree with the configured base_path. Worktrees not under it are\n" +
			"moved (git worktree move) and worktrees created outside wtm are adopted (their parent\n" +
			"recorded so `wtm sync` works). Pass --to to change base_path and move existing worktrees\n" +
			"to the new location. Dirty or locked worktrees are skipped unless --force; an occupied\n" +
			"target path is never overwritten.",
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

	params := worktree.RelocateParams{
		ProjectDir:     cfg.ProjectDir,
		StateDir:       cfg.StateDir,
		Config:         cfg.Config,
		TargetBasePath: resolveTargetBasePath(to, cfg),
		BaseBranch:     resolveBase("", cfg),
		Force:          force,
		DryRun:         dryRun,
	}

	interactive := rules.IsHumanFormat(format)
	// canPrompt gates the confirmation wizard: a human format on a real terminal.
	// A piped/non-TTY human run cannot show it, so it must be driven by flags.
	canPrompt := interactive && term.IsTerminal(int(os.Stdin.Fd()))

	// Without a terminal to confirm and without --yes to run unattended, relocate
	// refuses to mutate rather than silently moving worktrees or opening a wizard
	// against a non-TTY stdin. --dry-run is always safe (no writes).
	if !canPrompt && !yes && !dryRun {
		return fmt.Errorf("relocate needs a terminal to confirm; re-run with --%s to proceed unattended", domain.FlagYes)
	}

	plan, err := worktree.PlanRelocate(params)
	if err != nil {
		return err
	}

	if canPrompt && !yes && !dryRun {
		// A single wizard: opt-in base_path edit, one parent picker per adopted
		// worktree, then a recap. Always reachable — relocate is the only command that
		// changes base_path, so it opens even with zero worktrees. --to fixes base_path
		// up front, so the edit steps are dropped then.
		res, werr := runRelocateWizard(runRelocateWizardParams{
			Cfg:             cfg,
			Plan:            plan,
			BaseBranch:      params.BaseBranch,
			CurrentBasePath: params.TargetBasePath,
			EditBasePath:    to == "",
			HasWork:         relocateHasWork(relocateHasWorkParams{Plan: plan, To: to, ConfigBasePath: cfg.Config.Project.Worktrees.BasePath}),
		})
		if werr != nil {
			return werr
		}
		if res.Aborted {
			return renderRelocateAborted(cmd)
		}
		if !res.Confirmed {
			// The recap was skipped: nothing would change.
			return renderEmptyRelocate(cmd, true)
		}
		params.TargetBasePath = res.BasePath
		params.Parents = res.Parents
	} else {
		// Non-interactive, --yes, or --dry-run: no wizard.
		if !rules.PlanHasWork(plan) {
			return renderEmptyRelocate(cmd, interactive)
		}
		if dryRun {
			return renderRelocateDryRun(renderDryRunParams{
				Cmd:         cmd,
				Params:      params,
				Plan:        plan,
				Interactive: interactive,
			})
		}
	}

	var result domain.RelocateResult
	err = components.RunLoading(components.LoadingParams{
		Message: "Relocating worktrees…",
		Animate: interactive,
		Work:    func() error { var e error; result, e = worktree.Relocate(params); return e },
	})
	if err != nil {
		return err
	}

	if interactive {
		output.Frame(cmd.OutOrStdout(), func(w io.Writer) {
			output.FormatRelocateResult(w, result)
		})
	} else if jsonErr := output.WriteRelocateResultJSON(cmd.OutOrStdout(), result); jsonErr != nil {
		return jsonErr
	}

	if rules.RelocateHasFailure(result) {
		return domain.ErrAborted
	}
	return nil
}

type runRelocateWizardParams struct {
	Cfg             shared.ConfigResult
	Plan            domain.RelocatePlan
	BaseBranch      string
	CurrentBasePath string
	// EditBasePath enables the base_path keep/change steps. False when --to already
	// fixed base_path.
	EditBasePath bool
	// HasWork reports whether applying would already do something at the current
	// base_path (moves/adoptions, or a --to that changes the config base_path).
	HasWork bool
}

type relocateHasWorkParams struct {
	Plan           domain.RelocatePlan
	To             string
	ConfigBasePath string
}

// relocateHasWork reports whether a relocate would change anything up front: there are
// worktrees to move/adopt, or --to sets a base_path different from the config's.
func relocateHasWork(params relocateHasWorkParams) bool {
	if rules.PlanHasWork(params.Plan) {
		return true
	}
	return params.To != "" && params.To != params.ConfigBasePath
}

// runRelocateWizard drives the base_path edit, adoption parent pickers, and final
// apply confirmation. The recap is rendered by a command-owned closure that
// re-projects the plan onto the chosen base_path (pure — execution re-plans against
// the filesystem), keeping plan and format logic out of the TUI.
func runRelocateWizard(params runRelocateWizardParams) (relocatetui.RunResult, error) {
	recapText := func(basePath string, parents map[string]string) string {
		plan := params.Plan
		previous := ""
		if basePath != params.CurrentBasePath {
			plan = rules.ReprojectRelocatePlan(rules.ReprojectRelocatePlanParams{
				Plan:       params.Plan,
				ProjectDir: params.Cfg.ProjectDir,
				BasePath:   basePath,
			})
			previous = params.CurrentBasePath
		}
		return output.SprintRelocateRecap(output.RelocateRecapParams{
			Plan:             plan,
			Parents:          parents,
			PreviousBasePath: previous,
		})
	}

	return relocatetui.RunWizard(relocatetui.RunParams{
		ProjectDir:       params.Cfg.ProjectDir,
		Adoptions:        rules.PlanAdoptions(params.Plan),
		Branches:         branchCandidates(params.Cfg.ProjectDir),
		BaseBranch:       params.BaseBranch,
		CurrentBasePath:  params.CurrentBasePath,
		EditBasePath:     params.EditBasePath,
		HasWork:          params.HasWork,
		ValidateBasePath: func(s string) error { return rules.ValidateRelocateTarget(s) },
		RecapText:        recapText,
	})
}

type renderDryRunParams struct {
	Cmd         *cobra.Command
	Params      worktree.RelocateParams
	Plan        domain.RelocatePlan
	Interactive bool
}

// renderRelocateDryRun closes the dry-run flow: in text mode it prints the
// grouped plan and notes that nothing changed; in JSON mode it emits the preview
// result (the service performs no writes when DryRun is set).
func renderRelocateDryRun(p renderDryRunParams) error {
	if p.Interactive {
		output.FrameStart(p.Cmd.ErrOrStderr())
		barred := output.Barred(p.Cmd.ErrOrStderr())
		output.FormatRelocatePlan(barred, p.Plan)
		output.Blank(barred)
		output.Message(barred, "Dry run — no changes made.")
		output.FrameEnd(p.Cmd.ErrOrStderr())
		return nil
	}
	result, err := worktree.Relocate(p.Params)
	if err != nil {
		return err
	}
	return output.WriteRelocateResultJSON(p.Cmd.OutOrStdout(), result)
}

func renderRelocateAborted(cmd *cobra.Command) error {
	output.Frame(cmd.OutOrStdout(), func(w io.Writer) {
		output.Message(w, "Aborted.")
	})
	return nil
}

func resolveTargetBasePath(to string, cfg shared.ConfigResult) string {
	if to != "" {
		return to
	}
	return cfg.Config.Project.Worktrees.BasePath
}

func renderEmptyRelocate(cmd *cobra.Command, interactive bool) error {
	if !interactive {
		return output.WriteRelocateResultJSON(cmd.OutOrStdout(), domain.RelocateResult{})
	}
	output.Frame(cmd.OutOrStdout(), func(w io.Writer) {
		output.Message(w, "All worktrees are already aligned with base_path.")
	})
	return nil
}
