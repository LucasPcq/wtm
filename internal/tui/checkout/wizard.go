// Package checkout builds the interactive multi-step wizard for `wtm checkout`
// (pull request → parent branch → env strategy) with breadcrumb and back nav.
package checkout

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/tui/branchrefresh"
	"github.com/LucasPcq/wtm/internal/tui/components"
	"github.com/LucasPcq/wtm/internal/tui/worktreepicker"
)

const (
	stepPR      = "Pull request"
	stepParent  = "Parent branch"
	stepEnv     = "Env strategy"
	stepConfirm = "Confirm"
)

// checkoutConfirm is the recap's action value.
const checkoutConfirm = "checkout"

// WizardParams holds inputs for the checkout wizard.
type WizardParams struct {
	// ProjectDir is the repo root, used to re-fetch branches on refresh.
	ProjectDir string
	// PRLoader fetches open PRs asynchronously. When set (no Preselected), the
	// wizard renders instantly and streams the list in with a loading callout.
	PRLoader worktreepicker.PRLoaderFunc
	// Preselected skips the PR-selection step (a number was given as argument).
	Preselected *domain.PRInfo
	// WorktreeBranches lists branches already checked out — their PRs are disabled.
	WorktreeBranches []string
	// ParentBranches are the parent-branch options (local + remote-tracking).
	ParentBranches []domain.BranchCandidate
	ConfigStrategy domain.EnvStrategy
	// IncludeParent / IncludeEnv add the corresponding step. Set to false when the
	// value is already fixed by a flag (--from / --env-from).
	IncludeParent bool
	IncludeEnv    bool
	// FromOverride / EnvOverride are the --from / --env-from flag values, used to
	// resolve the effective source and env for the env-fallback confirmation when
	// their step is not shown.
	FromOverride string
	EnvOverride  string
	// IsolationApplies says run.toml declares something a worktree isolates. The
	// step is posed when it applies and --isolation did not answer it; the recap
	// names the answer either way.
	IsolationApplies  bool
	IsolationOverride domain.Isolation
	IsolationDefault  domain.Isolation
	// EnvFallback, when set, adds a conditional confirmation after the env step:
	// given the resolved source and env override it decides whether the "parent"
	// strategy falls back to copying .env from main. Injected by the command layer.
	EnvFallback func(source, envOverride string) (show bool, params components.NewConfirmParams)
	// Target, when set, classifies a branch name so the recap can say the PR's
	// branch will be reused rather than checked out fresh. Injected by the command
	// layer so this package stays free of git logic. Does not fetch — reuse is
	// knowable, and worth saying, before the origin fetch that follows the recap.
	Target func(branch string) domain.BranchTarget
}

// WizardResult holds the answers from the checkout wizard. FromBranch and
// EnvFromOverride are empty when their step was not included.
type WizardResult struct {
	PR              domain.PRInfo
	FromBranch      string
	EnvFromOverride string
	Isolation       domain.Isolation
}

// RunWizard displays the interactive checkout wizard. Returns ErrUserAborted on
// Esc at the first step.
func RunWizard(params WizardParams) (WizardResult, error) {
	if params.Preselected == nil {
		return runPicker(params)
	}
	return runPreselected(params)
}

// runPreselected runs the sync parent+env wizard for a known PR.
func runPreselected(params WizardParams) (WizardResult, error) {
	holder := &params.ParentBranches

	var steps []components.Step
	if params.IncludeParent {
		steps = append(steps, parentStep(holder, params.Preselected.BaseBranch))
	}
	if params.IncludeEnv {
		steps = append(steps, envStep(params.ConfigStrategy))
	}
	if includesIsolation(params) {
		steps = append(steps, components.IsolationStep(params.IsolationDefault))
	}

	if len(steps) == 0 {
		// Everything is fixed by flags — nothing to review, so no wizard (and no
		// recap): this is effectively a non-interactive checkout.
		return WizardResult{PR: *params.Preselected, Isolation: resolveIsolation(nil, params)}, nil
	}

	preselected := *params.Preselected
	steps = append(steps, recapStep(params, func([]components.Step) string {
		return prDisplay(preselected)
	}, func([]components.Step) string {
		return preselected.Branch
	}))

	wiz := components.NewWizardWithParams(components.WizardParams{
		Steps: steps,
		OnMsg: branchrefresh.Handler(params.ProjectDir, holder),
	})
	final, err := runProgram(wiz)
	if err != nil {
		return WizardResult{}, err
	}
	if stepConfirmValue(final.Steps()) == domain.WizardCancelValue {
		return WizardResult{}, domain.ErrUserAborted
	}

	res := extractStepValues(final.Steps(), params)
	res.PR = *params.Preselected
	return res, nil
}

// runPicker runs the async wizard: a PR-selection step that streams PRs in,
// followed by the parent (rebuilt from the chosen PR) and env steps.
func runPicker(params WizardParams) (WizardResult, error) {
	var loadedPRs []domain.PRInfo
	holder := &params.ParentBranches

	steps := []components.Step{prStep(params)}
	if params.IncludeParent {
		steps = append(steps, components.Step{
			Name:       stepParent,
			Summary:    selectValueSummary,
			Model:      components.NewSelectList(components.NewSelectListParams{Title: stepParent}),
			CanRefresh: true,
			Build: func(prev []components.Step) any {
				base := baseBranchForSelection(prev, loadedPRs)
				return parentList(*holder, base)
			},
		})
	}
	if params.IncludeEnv {
		steps = append(steps, envStep(params.ConfigStrategy))
	}
	if includesIsolation(params) {
		steps = append(steps, components.IsolationStep(params.IsolationDefault))
	}
	selectedPR := func(prev []components.Step) (domain.PRInfo, bool) {
		sl, ok := stepModel(prev, stepPR)
		if !ok {
			return domain.PRInfo{}, false
		}
		num, _ := strconv.Atoi(sl.Value())
		return findPR(loadedPRs, num)
	}
	steps = append(steps, recapStep(params, func(prev []components.Step) string {
		if pr, found := selectedPR(prev); found {
			return prDisplay(pr)
		}
		return ""
	}, func(prev []components.Step) string {
		if pr, found := selectedPR(prev); found {
			return pr.Branch
		}
		return ""
	}))

	refresh := branchrefresh.Handler(params.ProjectDir, holder)
	wiz := components.NewWizardWithParams(components.WizardParams{
		Steps:       steps,
		InitCmd:     tea.Batch(worktreepicker.PRLoadCmd(params.PRLoader), branchrefresh.Cmd(params.ProjectDir)),
		Loading:     true,
		LoadingText: worktreepicker.LoadingPRsText,
		OnMsg: func(w *components.WizardModel, msg tea.Msg) (tea.Cmd, bool) {
			if cmd, handled := refresh(w, msg); handled {
				return cmd, true
			}
			loaded, ok := msg.(worktreepicker.PRsLoadedMsg)
			if !ok {
				return nil, false
			}
			loadedPRs = loaded.PRs
			items := buildPRItems(loaded.PRs, params.WorktreeBranches)
			w.UpdateStepModel(0, func(model any) any {
				sl, ok := model.(components.SelectListModel)
				if !ok {
					return model
				}
				sl.SetItems(items)
				return sl
			})
			w.SetLoading(false)
			w.SetBanner(pickerBanner(loaded))
			return nil, true
		},
	})

	final, err := runProgram(wiz)
	if err != nil {
		return WizardResult{}, err
	}
	if stepConfirmValue(final.Steps()) == domain.WizardCancelValue {
		return WizardResult{}, domain.ErrUserAborted
	}

	res := extractStepValues(final.Steps(), params)
	if sl, ok := stepModel(final.Steps(), stepPR); ok {
		num, _ := strconv.Atoi(sl.Value())
		if pr, found := findPR(loadedPRs, num); found {
			res.PR = pr
		}
	}
	return res, nil
}

func runProgram(wiz components.WizardModel) (components.WizardModel, error) {
	finalModel, err := tea.NewProgram(wiz).Run()
	if err != nil {
		return components.WizardModel{}, fmt.Errorf("wizard: %w", err)
	}
	final, ok := finalModel.(components.WizardModel)
	if !ok || final.Aborted() {
		return components.WizardModel{}, domain.ErrUserAborted
	}
	return final, nil
}

func extractStepValues(steps []components.Step, params WizardParams) WizardResult {
	r := WizardResult{Isolation: resolveIsolation(steps, params)}
	if sl, ok := stepModel(steps, stepParent); ok {
		r.FromBranch = sl.Value()
	}
	if sl, ok := stepModel(steps, stepEnv); ok {
		r.EnvFromOverride = sl.Value()
	}
	return r
}

func stepModel(steps []components.Step, name string) (components.SelectListModel, bool) {
	for _, s := range steps {
		if s.Name != name {
			continue
		}
		sl, ok := s.Model.(components.SelectListModel)
		return sl, ok
	}
	return components.SelectListModel{}, false
}

// pickerBanner returns the status box shown once the PR fetch completes: the
// GitHub-connection hint, or a "no PRs" notice when connected but empty.
func pickerBanner(loaded worktreepicker.PRsLoadedMsg) components.WizardBanner {
	if banner := worktreepicker.GHBanner(loaded.Conn); banner.Title != "" {
		return banner
	}
	if len(loaded.PRs) == 0 {
		return components.WizardBanner{Title: "No open pull requests"}
	}
	return components.WizardBanner{}
}

func prStep(params WizardParams) components.Step {
	return components.Step{
		Name: stepPR,
		Model: components.NewSelectList(components.NewSelectListParams{
			Title:       "Select a pull request to checkout",
			Description: "Linked PRs are disabled — use `wtm go <branch>` to enter them",
		}),
		Summary: func(m any) string {
			sl, ok := m.(components.SelectListModel)
			if !ok || sl.Value() == "" {
				return ""
			}
			return "#" + sl.Value()
		},
	}
}

func parentStep(holder *[]domain.BranchCandidate, base string) components.Step {
	return components.Step{
		Name:       stepParent,
		Summary:    selectValueSummary,
		Model:      parentList(*holder, base),
		Build:      func([]components.Step) any { return parentList(*holder, base) },
		CanRefresh: true,
	}
}

func parentList(branches []domain.BranchCandidate, base string) components.SelectListModel {
	return components.NewSelectList(components.NewSelectListParams{
		Title:       stepParent,
		Description: "Branch this PR is rebased onto by `wtm sync` (defaults to the PR base)",
		Items:       buildBranchItems(branches, base),
	})
}

func baseBranchForSelection(prev []components.Step, prs []domain.PRInfo) string {
	sl, ok := stepModel(prev, stepPR)
	if !ok {
		return ""
	}
	num, _ := strconv.Atoi(sl.Value())
	if pr, found := findPR(prs, num); found {
		return pr.BaseBranch
	}
	return ""
}

// recapStep builds the final, unconditional recap for checkout: it recaps the PR,
// parent, and env, folds the env fallback into a ⚠ line, and offers
// "Yes, checkout" then the constant "No, cancel". prLabel resolves the chosen PR's
// display text and prBranch its branch name (both fixed for a preselected PR, or
// looked up from the picked number).
func recapStep(params WizardParams, prLabel, prBranch func(prev []components.Step) string) components.Step {
	return components.RecapStep(components.RecapStepParams{
		Name: stepConfirm,
		Build: func(prev []components.Step) components.RecapContent {
			return components.RecapContent{
				Description: buildCheckoutRecap(prev, params, prLabel, prBranch),
				Actions: []components.SelectItem{
					{Label: "Yes, checkout", Value: checkoutConfirm},
				},
			}
		},
	})
}

// buildCheckoutRecap recaps the selections with a ⚠ line for the env fallback and a
// reuse line when the PR's branch already exists locally — the one thing a user
// confirming "checkout PR #42" would otherwise not know before it is checked out.
func buildCheckoutRecap(prev []components.Step, params WizardParams, prLabel, prBranch func(prev []components.Step) string) string {
	var lines []string
	if pr := prLabel(prev); pr != "" {
		lines = append(lines, "PR:      "+pr)
	}
	if params.Target != nil {
		if b := prBranch(prev); b != "" && params.Target(b).State == domain.BranchTargetExisting {
			lines = append(lines, "Branch:  "+b+domain.BranchReusedSuffix)
		}
	}
	source := resolveSource(prev, params.FromOverride, params.Preselected)
	if source != "" {
		lines = append(lines, "Parent:  "+source)
	}
	env := resolveEnv(prev, params.EnvOverride)
	envLabel := env
	if envLabel == "" {
		envLabel = domain.SummaryConfigDefault
	}
	lines = append(lines, "Env:     "+envLabel)
	if params.IsolationApplies {
		lines = append(lines, domain.RecapFieldIsolation+rules.IsolationSummary(resolveIsolation(prev, params)))
	}

	if params.EnvFallback != nil {
		if show, p := params.EnvFallback(source, env); show && p.Warning != "" {
			lines = append(lines, "", "⚠ "+p.Warning)
		}
	}
	return strings.Join(lines, "\n")
}

func includesIsolation(params WizardParams) bool {
	return params.IsolationApplies && params.IsolationOverride == ""
}

// resolveIsolation is the step's answer, else the flag, else the project's.
func resolveIsolation(steps []components.Step, params WizardParams) domain.Isolation {
	if !includesIsolation(params) {
		return rules.EffectiveIsolation(rules.FirstIsolation(params.IsolationOverride, params.IsolationDefault))
	}
	return components.IsolationAnswer(steps, params.IsolationDefault)
}

// prDisplay renders a PR as "#<number> <title>" for the recap.
func prDisplay(pr domain.PRInfo) string {
	return fmt.Sprintf("#%d %s", pr.Number, pr.Title)
}

// stepConfirmValue reads the value chosen on the recap step.
func stepConfirmValue(steps []components.Step) string {
	if sl, ok := stepModel(steps, stepConfirm); ok {
		return sl.Value()
	}
	return ""
}

// resolveSource resolves the effective parent/source branch for the env-fallback
// check: the --from override, else the chosen parent step, else the preselected
// PR's base branch.
func resolveSource(prev []components.Step, fromOverride string, preselected *domain.PRInfo) string {
	if fromOverride != "" {
		return fromOverride
	}
	if sl, ok := stepModel(prev, stepParent); ok && sl.Value() != "" {
		return sl.Value()
	}
	if preselected != nil {
		return preselected.BaseBranch
	}
	return ""
}

// resolveEnv resolves the effective env override: the --env-from override, else
// the chosen env step.
func resolveEnv(prev []components.Step, envOverride string) string {
	if envOverride != "" {
		return envOverride
	}
	if sl, ok := stepModel(prev, stepEnv); ok {
		return sl.Value()
	}
	return ""
}

func envStep(strategy domain.EnvStrategy) components.Step {
	return components.Step{
		Name: stepEnv,
		Model: components.NewSelectList(components.NewSelectListParams{
			Title:       stepEnv,
			Description: "How to provision .env files in the new worktree",
			Items:       buildEnvItems(strategy),
		}),
		Summary: func(m any) string {
			sl, ok := m.(components.SelectListModel)
			if !ok {
				return ""
			}
			if sl.Value() == "" {
				return domain.SummaryConfigDefault
			}
			return sl.Value()
		},
	}
}

func selectValueSummary(m any) string {
	sl, ok := m.(components.SelectListModel)
	if !ok {
		return ""
	}
	return sl.Value()
}

func findPR(prs []domain.PRInfo, number int) (domain.PRInfo, bool) {
	for _, p := range prs {
		if p.Number == number {
			return p, true
		}
	}
	return domain.PRInfo{}, false
}

// buildPRItems builds the picker items, disabling PRs whose branch already has a
// local worktree (badge "linked") or that come from a fork (badge "fork").
func buildPRItems(prs []domain.PRInfo, worktreeBranches []string) []components.SelectItem {
	linked := make(map[string]bool, len(worktreeBranches))
	for _, b := range worktreeBranches {
		linked[b] = true
	}

	items := make([]components.SelectItem, 0, len(prs))
	for _, p := range prs {
		item := components.SelectItem{
			Label: fmt.Sprintf("#%-4d  %-40s  %s", p.Number, truncate(p.Title, 40), p.Author),
			Value: strconv.Itoa(p.Number),
		}
		switch {
		case linked[p.Branch]:
			item.Disabled = true
			item.Badges = []components.Badge{{Text: domain.BadgeTextLinked, Variant: components.BadgeNeutral}}
		case p.IsFork:
			item.Disabled = true
			item.Badges = []components.Badge{{Text: domain.BadgeTextFork, Variant: components.BadgeWarning}}
		}
		items = append(items, item)
	}
	return items
}

// buildBranchItems lists candidate parent branches with the PR base branch
// pinned first as the pre-selected default and remote-tracking branches grouped
// after a separator.
func buildBranchItems(branches []domain.BranchCandidate, base string) []components.SelectItem {
	return components.BranchItems(components.BranchItemsParams{
		Candidates:   branches,
		Pinned:       base,
		PinnedSuffix: domain.PinnedSuffixBase,
	})
}

func buildEnvItems(strategy domain.EnvStrategy) []components.SelectItem {
	return []components.SelectItem{
		{Label: "Use config default (" + string(strategy) + ")", Value: ""},
		{Label: "example — copy .env.example → .env", Value: string(domain.EnvStrategyExample)},
		{Label: "main — copy .env from the main checkout", Value: string(domain.EnvStrategyMain)},
		{Label: "parent — copy .env from source worktree", Value: string(domain.EnvStrategyParent)},
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}
