// Package checkout implements the wtm checkout command: create a worktree from
// an existing GitHub pull request.
package checkout

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/branch"
	ghservice "github.com/LucasPcq/wtm/internal/service/github"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	checkoutwizard "github.com/LucasPcq/wtm/internal/tui/checkout"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// NewCmd creates the wtm checkout command.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdCheckout + " [number]",
		Short: "Create a worktree from an existing pull request",
		Long: "Create a worktree from a pull request.\n" +
			"A local branch of the PR's name is checked out as-is, keeping commits you never\n" +
			"pushed; interactive runs offer to fast-forward it when it is behind origin.\n" +
			"Without arguments, shows an interactive picker of open PRs.",
		Args: cobra.MaximumNArgs(1),
		RunE: runCheckout,
	}

	cmd.Flags().Bool(domain.FlagReview, false, "Show only PRs where you are requested as reviewer")
	cmd.Flags().Bool(domain.FlagMine, false, "Show only your PRs")
	cmd.Flags().String(domain.FlagFrom, "", "Parent branch for sync (defaults to the PR base branch)")
	cmd.Flags().String(domain.FlagEnvFrom, "", "Override env strategy (example, main, parent)")
	cmd.Flags().BoolP(domain.FlagYes, "y", false, "Skip all prompts; resolve every decision from flags and safe defaults (PR number required)")
	shared.AddOutputFlag(cmd)

	return cmd
}

func runCheckout(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	result, err := shared.LoadConfig(cmd, dir)
	if err != nil {
		return err
	}

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	fromOverride, _ := cmd.Flags().GetString(domain.FlagFrom)
	envOverride, _ := cmd.Flags().GetString(domain.FlagEnvFrom)
	yes, _ := cmd.Flags().GetBool(domain.FlagYes)

	if format == domain.OutputJSON && !yes {
		return fmt.Errorf("--output json requires --%s (prompts cannot run in JSON mode)", domain.FlagYes)
	}

	// --yes runs prompt-free: parent defaults to the PR base branch, env to the
	// config strategy, and the env-fallback confirmation is skipped. It behaves like
	// a non-interactive run (a PR number is required) while still framing human output.
	opts := checkoutOptions{
		jsonMode:     format == domain.OutputJSON,
		interactive:  rules.IsHumanFormat(format) && term.IsTerminal(int(os.Stdin.Fd())) && !yes,
		fromOverride: fromOverride,
		envOverride:  envOverride,
	}

	if len(args) == 1 {
		number, parseErr := strconv.Atoi(args[0])
		if parseErr != nil || number <= 0 {
			return fmt.Errorf("invalid PR number %q", args[0])
		}
		return checkoutByNumber(cmd, result, number, opts)
	}

	return checkoutInteractive(cmd, result, opts)
}

// checkoutOptions carries the resolved invocation mode and overrides.
type checkoutOptions struct {
	jsonMode     bool
	interactive  bool
	fromOverride string
	envOverride  string
}

// checkoutByNumber handles `wtm checkout <number>`. The loading box owns its own
// spacing; the persistent top padding comes from the framed result.
func checkoutByNumber(cmd *cobra.Command, result shared.ConfigResult, number int, opts checkoutOptions) error {
	var p domain.PRInfo
	err := components.RunLoading(components.LoadingParams{
		Message: "Fetching PR…",
		Animate: !opts.jsonMode,
		Work: func() error {
			var e error
			p, e = ghservice.GetPRDetail(ghservice.GetPRDetailParams{
				ProjectDir: result.ProjectDir,
				Number:     number,
			})
			return e
		},
	})
	if err != nil {
		return fmt.Errorf("fetch PR: %w", err)
	}

	if err := rules.ValidatePRForCheckout(p); err != nil {
		return err
	}
	parentBranches := parentBranchCandidates(result.ProjectDir)

	parent, env, aborted, wizardRan, err := resolveParentAndEnv(resolveParams{
		result:         result,
		pr:             p,
		parentBranches: parentBranches,
		opts:           opts,
	})
	if err != nil || aborted {
		return err
	}

	// When the wizard ran it already hosted the env-fallback confirmation; only
	// the no-wizard interactive path (both --from and --env-from given) still needs
	// the standalone confirm.
	return createFromPR(cmd, result, createFromPRParams{pr: p, parent: parent, env: env, jsonMode: opts.jsonMode, interactive: opts.interactive, envConfirmed: wizardRan})
}

// checkoutInteractive handles `wtm checkout` with no number: it renders the
// multi-step wizard instantly and streams open PRs in asynchronously.
func checkoutInteractive(cmd *cobra.Command, result shared.ConfigResult, opts checkoutOptions) error {
	if !opts.interactive {
		return fmt.Errorf("PR number required without an interactive terminal (or when --yes is set)")
	}

	parentBranches := parentBranchCandidates(result.ProjectDir)

	dir := result.ProjectDir
	review, _ := cmd.Flags().GetBool(domain.FlagReview)
	mine, _ := cmd.Flags().GetBool(domain.FlagMine)
	filter := rules.PRFilterFor(rules.PRFilterParams{Review: review, Mine: mine})

	res, err := checkoutwizard.RunWizard(checkoutwizard.WizardParams{
		ProjectDir:       dir,
		PRLoader:         func() ([]domain.PRInfo, domain.GHConnection) { return shared.LoadPRsFiltered(dir, filter) },
		WorktreeBranches: worktreeBranches(dir),
		ParentBranches:   parentBranches,
		ConfigStrategy:   result.Config.Project.Env.Strategy,
		IncludeParent:    opts.fromOverride == "",
		IncludeEnv:       opts.envOverride == "",
		FromOverride:     opts.fromOverride,
		EnvOverride:      opts.envOverride,
		EnvFallback:      shared.EnvFallbackDecider(dir, result.Config),
		Target: func(b string) domain.BranchTarget {
			return branch.Target(branch.BranchParams{ProjectDir: dir, Branch: b})
		},
	})
	if err != nil {
		if errors.Is(err, domain.ErrUserAborted) {
			return nil
		}
		return err
	}

	p := res.PR
	if p.Number == 0 {
		return nil
	}
	if err := rules.ValidatePRForCheckout(p); err != nil {
		return err
	}

	parent := rules.FirstNonEmpty(opts.fromOverride, res.FromBranch, p.BaseBranch)
	env := rules.FirstNonEmpty(opts.envOverride, res.EnvFromOverride)

	// The env-fallback confirmation ran inside the wizard.
	return createFromPR(cmd, result, createFromPRParams{pr: p, parent: parent, env: env, jsonMode: opts.jsonMode, interactive: opts.interactive, envConfirmed: true})
}

// resolveParams holds inputs for resolving parent/env for a known PR.
type resolveParams struct {
	result         shared.ConfigResult
	pr             domain.PRInfo
	parentBranches []domain.BranchCandidate
	opts           checkoutOptions
}

// resolveParentAndEnv determines the parent branch and env strategy for a known
// PR, running the wizard for whatever the flags left unset (interactive only).
// wizardRan reports whether the wizard ran — and thus already hosted the
// env-fallback confirmation.
func resolveParentAndEnv(params resolveParams) (parent, env string, aborted, wizardRan bool, err error) {
	parent = params.opts.fromOverride
	env = params.opts.envOverride

	needParent := parent == ""
	needEnv := env == ""

	if params.opts.interactive && (needParent || needEnv) {
		wizardRan = true
		pr := params.pr
		res, runErr := checkoutwizard.RunWizard(checkoutwizard.WizardParams{
			ProjectDir:     params.result.ProjectDir,
			Preselected:    &pr,
			ParentBranches: params.parentBranches,
			ConfigStrategy: params.result.Config.Project.Env.Strategy,
			IncludeParent:  needParent,
			IncludeEnv:     needEnv,
			FromOverride:   params.opts.fromOverride,
			EnvOverride:    params.opts.envOverride,
			EnvFallback:    shared.EnvFallbackDecider(params.result.ProjectDir, params.result.Config),
			Target: func(b string) domain.BranchTarget {
				return branch.Target(branch.BranchParams{ProjectDir: params.result.ProjectDir, Branch: b})
			},
		})
		if runErr != nil {
			if errors.Is(runErr, domain.ErrUserAborted) {
				return "", "", true, wizardRan, nil
			}
			return "", "", false, wizardRan, runErr
		}
		if needParent {
			parent = res.FromBranch
		}
		if needEnv {
			env = res.EnvFromOverride
		}
	}

	if parent == "" {
		parent = params.pr.BaseBranch
	}
	return parent, env, false, wizardRan, nil
}

// createFromPRParams holds inputs for the final worktree creation step.
type createFromPRParams struct {
	pr          domain.PRInfo
	parent      string
	env         string
	jsonMode    bool
	interactive bool
	// envConfirmed is true when the env-fallback confirmation already ran inside
	// the wizard; only the no-wizard interactive path confirms it here.
	envConfirmed bool
}

func createFromPR(cmd *cobra.Command, result shared.ConfigResult, params createFromPRParams) error {
	p := params.pr

	if params.interactive && !params.envConfirmed {
		if show, cp := shared.EnvFallbackDecider(result.ProjectDir, result.Config)(params.parent, params.env); show {
			if confirmed, _ := components.RunStandaloneConfirm(components.NewConfirm(cp)); !confirmed {
				return nil
			}
		}
	}

	fetchErr := components.RunLoading(components.LoadingParams{
		Message: "Fetching branch from origin…",
		Animate: !params.jsonMode,
		Work: func() error {
			return infra.FetchBranch(infra.FetchBranchParams{
				ProjectDir: result.ProjectDir,
				Branch:     p.Branch,
			})
		},
	})
	if fetchErr != nil {
		return fetchErr
	}

	// A local branch of the PR's name is not a conflict: the worktree checks it
	// out as-is, keeping commits that were never pushed. Only a branch another
	// worktree already holds is refused, by worktree.Create.
	target := branch.Target(branch.BranchParams{ProjectDir: result.ProjectDir, Branch: p.Branch})
	reused := target.State == domain.BranchTargetExisting

	// The start-point applies only to a branch git has to create. Reusing one is
	// never destructive, so it needs no confirmation — but a behind-only branch is
	// worth offering to update, interactively only (--yes / JSON never touch refs).
	startPoint := domain.RemoteBranchPrefix + p.Branch
	if reused {
		startPoint = ""
		if params.interactive {
			updated, ok := reconcileReusedBranch(reconcileReusedBranchParams{ProjectDir: result.ProjectDir, Target: target})
			if !ok {
				return nil
			}
			target = updated
		}
	}

	if !params.jsonMode {
		output.Loading(cmd.ErrOrStderr(), fmt.Sprintf("Creating worktree %s…", p.Branch))
	}
	createResult, err := worktree.Create(domain.CreateParams{
		ProjectDir:      result.ProjectDir,
		StateDir:        result.StateDir,
		Branch:          p.Branch,
		FromBranch:      startPoint,
		SourceBranch:    params.parent,
		Config:          result.Config,
		EnvFromOverride: params.env,
		SkipHooks:       true,
	})
	if err != nil {
		return err
	}

	// on_create hooks as a distinct, titled phase (shared with create/extract).
	// A reused branch has no start-point, so the hooks see its recorded parent.
	if hookErr := shared.RunCreateHooksPhase(shared.CreateHooksPhaseParams{
		StateDir:     result.StateDir,
		Cmd:          cmd,
		ShowHeader:   !params.jsonMode,
		ProjectDir:   result.ProjectDir,
		WorktreePath: createResult.Path,
		Branch:       p.Branch,
		FromBranch:   rules.FirstNonEmpty(startPoint, params.parent),
		Hooks:        result.Config.Project.Hooks.OnCreate,
	}); hookErr != nil {
		return hookErr
	}

	if params.jsonMode {
		return output.WritePRCheckoutJSON(cmd.OutOrStdout(), output.PRCheckoutJSON{
			Number:         p.Number,
			Branch:         p.Branch,
			Path:           createResult.Path,
			Author:         p.Author,
			URL:            p.URL,
			Draft:          p.Draft,
			ExistingBranch: createResult.ExistingBranch,
			OriginState:    createResult.OriginState,
		})
	}

	output.Frame(cmd.OutOrStdout(), func(w io.Writer) {
		output.Success(w, fmt.Sprintf("Checked out PR #%d (%s) at %s", p.Number, p.Branch, createResult.Path))
		if createResult.ExistingBranch {
			note := shared.ReusedBranchNote(shared.ReusedBranchNoteParams{
				Branch: target.Branch,
				Ahead:  target.AheadBehind.Ahead,
				Behind: target.AheadBehind.Behind,
			})
			if note.Warning {
				output.Warning(w, note.Text)
			} else {
				output.Message(w, note.Text)
			}
		}
		output.GoHint(w, fmt.Sprintf(domain.GoCommandFmt, p.Branch))
	})
	return nil
}

// reconcileReusedBranchParams holds inputs for reconcileReusedBranch.
type reconcileReusedBranchParams struct {
	ProjectDir string
	Target     domain.BranchTarget
}

// reconcileReusedBranch offers to update a reused local branch that is behind
// origin, mirroring create's fast-forward offer, and returns the branch's state
// afterwards so the conclusion reports what is actually checked out. Declining
// proceeds with the local branch as-is; only a failed fast-forward whose recovery
// is declined cancels the checkout (ok=false). A diverged branch is never
// rewritten — the local commits win and the user is told what that means.
// Interactive runs only.
func reconcileReusedBranch(p reconcileReusedBranchParams) (updated domain.BranchTarget, ok bool) {
	target := p.Target
	if !rules.ShouldOfferFastForward(target.Origin) {
		return target, true
	}
	confirmed, _ := components.RunStandaloneConfirm(components.NewConfirm(components.NewConfirmParams{
		Title:       fmt.Sprintf(domain.SourceFastForwardPrompt, target.Branch, target.AheadBehind.Behind),
		Description: domain.SourceFastForwardDescription,
		DefaultYes:  true,
	}))
	if !confirmed {
		return target, true
	}

	ffErr := components.RunLoading(components.LoadingParams{
		Message: fmt.Sprintf(domain.SourceFastForwardLoadingFmt, target.Branch),
		Animate: true,
		Work: func() error {
			return branch.FastForwardToOrigin(branch.BranchParams{ProjectDir: p.ProjectDir, Branch: target.Branch})
		},
	})
	if ffErr == nil {
		return branch.Target(branch.BranchParams{ProjectDir: p.ProjectDir, Branch: target.Branch}), true
	}

	proceed, _ := components.RunStandaloneConfirm(components.NewConfirm(components.NewConfirmParams{
		Title:      fmt.Sprintf(domain.SourceProceedStalePrompt, target.Branch, target.AheadBehind.Behind),
		Warning:    fmt.Sprintf(domain.SourceProceedStaleWarning, ffErr),
		DefaultYes: false,
	}))
	return target, proceed
}

func worktreeBranches(projectDir string) []string {
	wts, err := infra.ListWorktrees(infra.ListWorktreesParams{ProjectDir: projectDir})
	if err != nil {
		return nil
	}
	branches := make([]string, 0, len(wts))
	for _, wt := range wts {
		branches = append(branches, wt.Branch)
	}
	return branches
}

// parentBranchCandidates lists the local and origin remote-tracking branches,
// tagged with their divergence, as the parent-branch options for the checkout
// wizard. Best effort: a listing failure yields an empty set rather than an error.
func parentBranchCandidates(projectDir string) []domain.BranchCandidate {
	return branch.Candidates(branch.ListParams{ProjectDir: projectDir})
}
