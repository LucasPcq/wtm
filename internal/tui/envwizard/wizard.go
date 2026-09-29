// Package envwizard assembles the unified interactive flow for `wtm env`: one
// wizard from worktree selection → single-screen drift resolution → recap, with a
// breadcrumb and Esc back-navigation throughout. The per-worktree drift is computed
// by the caller (command layer) and passed in, so this package stays free of any
// service/git import.
package envwizard

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

const (
	applyAction = "apply"
	// applyVerbatimAction declines the port pass for good: the worktree is
	// recorded verbatim, so its jobs run on the ports its .env keeps.
	applyVerbatimAction = "apply-verbatim"
)

// RunParams holds the wizard inputs. DiffByBranch is the precomputed per-worktree
// drift (keyed by branch); Candidates are the worktrees offered when no PresetBranch
// is given. PresetBranch skips the selection step (a worktree was passed as an arg).
type RunParams struct {
	Candidates   []domain.WorktreeStatus
	PresetBranch string
	DiffByBranch map[string][]domain.EnvFileResult
	// PortsByBranch is the [[env_port]] pass each worktree would get. It is not a
	// decision the wizard offers — it rides along with the apply — which is
	// exactly why the recap has to name it: discovering it afterwards is a
	// surprise, and a surprise about a file the user did not agree to touch.
	PortsByBranch map[string]domain.EnvPortPlan
	// AdoptionByBranch holds the worktrees created before the isolation choice
	// existed. Adopting it is a question of its own, since it moves the worktree
	// off the compose project whose volumes hold its data.
	AdoptionByBranch map[string]domain.IsolationAdoptionPlan
}

// Result is the wizard outcome: the chosen worktree and its per-file decisions.
type Result struct {
	Branch    string
	Decisions []components.EnvFileDecision
	// Verbatim is the user choosing to apply without the port pass, which
	// records the worktree verbatim.
	Verbatim bool
	Adopt    bool
}

// Run drives the unified wizard and returns the branch + collected decisions.
// Returns domain.ErrUserAborted on Esc (first step) or the recap "No, cancel" row.
func Run(params RunParams) (Result, error) {
	steps := make([]components.Step, 0, 3)

	worktreeIdx := -1
	if params.PresetBranch == "" {
		worktreeIdx = len(steps)
		steps = append(steps, worktreeStep(params))
	}

	branchOf := func(prev []components.Step) string {
		if params.PresetBranch != "" {
			return params.PresetBranch
		}
		return selectValue(prev, worktreeIdx)
	}

	adoptIdx := -1
	if (params.PresetBranch == "" && len(params.AdoptionByBranch) > 0) || params.AdoptionByBranch[params.PresetBranch].Pending {
		adoptIdx = len(steps)
		steps = append(steps, adoptionStep(params, branchOf))
	}

	// The wizard never auto-skips its first step, which is where a preset branch
	// puts the resolver: with nothing to decide it would sit there empty, so it is
	// left out here instead.
	resolveIdx := -1
	if len(steps) > 0 || !resolveModel(params, params.PresetBranch).Empty() {
		resolveIdx = len(steps)
		steps = append(steps, resolveStep(params, branchOf))
	}

	recapIdx := len(steps)
	steps = append(steps, recapStep(recapStepParams{
		BranchOf:      branchOf,
		ResolveIdx:    resolveIdx,
		AdoptIdx:      adoptIdx,
		PortsByBranch: params.PortsByBranch,
	}))

	final, err := components.RunWizard(components.RunWizardParams{
		Steps:    steps,
		Stderr:   true,
		ErrLabel: "env wizard",
	})
	if err != nil {
		return Result{}, err
	}

	done := final.Steps()
	action := selectValue(done, recapIdx)
	if action == domain.WizardCancelValue {
		return Result{}, domain.ErrUserAborted
	}

	res := Result{
		Branch:   branchOf(done),
		Verbatim: action == applyVerbatimAction,
		Adopt:    adoptIdx >= 0 && selectValue(done, adoptIdx) == domain.IsolationAdoptValue,
	}
	if m, ok := stepModel(done, resolveIdx).(components.EnvResolveModel); ok {
		res.Decisions = m.Decisions()
	}
	return res, nil
}

// worktreeStep is the selection list, each row badged with its env-drift status
// (not git clean/dirty).
func worktreeStep(params RunParams) components.Step {
	items := make([]components.SelectItem, 0, len(params.Candidates))
	for _, c := range params.Candidates {
		badge := driftBadge(params.DiffByBranch[c.Branch])
		var tags []components.Badge
		if c.IsParent {
			tags = append(tags, components.Badge{Text: "parent", Variant: components.BadgeAccent})
		}
		items = append(items, components.SelectItem{
			Label:  c.Branch,
			Value:  c.Branch,
			Badges: tags,
			Status: &badge,
		})
	}
	return components.Step{
		Name:    "Select worktree",
		Model:   components.NewSelectList(components.NewSelectListParams{Title: "Select a worktree to reconcile", Items: items}),
		Summary: components.SelectSummary,
	}
}

// resolveStep builds the single-screen resolver for the selected worktree's drift,
// auto-skipping when there is nothing to decide (only safe additions / in sync).
func resolveStep(params RunParams, branchOf func([]components.Step) string) components.Step {
	return components.Step{
		Name:    "Resolve",
		Model:   components.NewEnvResolve(components.NewEnvResolveParams{}),
		Callout: true, // renders the glossary (the model's desc) as a legend callout
		Build: func(prev []components.Step) any {
			return resolveModel(params, branchOf(prev))
		},
		AutoSkip: func(w components.WizardModel) bool {
			m, ok := w.CurrentStepModel().(components.EnvResolveModel)
			return ok && m.Empty()
		},
		SkipReason: func() string { return "only safe additions" },
		Summary:    components.EnvResolveSummary,
	}
}

// adoptionStep asks a worktree created before the isolation choice whether to
// adopt it. Keeping it as is opens under the cursor: adopting leaves its data
// behind in the compose project it runs under today.
func adoptionStep(params RunParams, branchOf func([]components.Step) string) components.Step {
	return components.Step{
		Name:  domain.IsolationAdoptStepName,
		Model: components.NewSelectList(components.NewSelectListParams{}),
		Build: func(prev []components.Step) any {
			branch := branchOf(prev)
			plan := params.AdoptionByBranch[branch]
			return components.NewSelectList(components.NewSelectListParams{
				Title:       fmt.Sprintf(domain.IsolationAdoptTitleFmt, branch),
				Description: domain.IsolationAdoptDescription,
				Items: []components.SelectItem{
					{Label: domain.IsolationAdoptKeepLabel, Value: domain.IsolationAdoptKeepValue},
					{Label: rules.IsolationAdoptOptionLabel(plan), Value: domain.IsolationAdoptValue, Danger: plan.ComposeProject != ""},
				},
			})
		},
		AutoSkip: func(w components.WizardModel) bool {
			return !params.AdoptionByBranch[branchOf(w.Steps())].Pending
		},
		Summary: func(model any) string {
			if components.SelectSummary(model) == domain.IsolationAdoptValue {
				return domain.IsolationAdoptSummary
			}
			return domain.IsolationAdoptKeptSummary
		},
	}
}

func resolveModel(params RunParams, branch string) components.EnvResolveModel {
	return components.NewEnvResolve(components.NewEnvResolveParams{
		Title:       "Resolve drift — " + branch,
		Description: components.EnvResolveGlossary(),
		Files:       params.DiffByBranch[branch],
	})
}

type recapStepParams struct {
	BranchOf      func([]components.Step) string
	ResolveIdx    int
	AdoptIdx      int
	PortsByBranch map[string]domain.EnvPortPlan
}

// recapStep restates the worktree, every decision (with values) and the port
// values the apply will shift, then offers "Yes, apply" / "No, cancel".
func recapStep(params recapStepParams) components.Step {
	return components.RecapStep(components.RecapStepParams{
		Name: "Review & apply",
		Build: func(prev []components.Step) components.RecapContent {
			branch := params.BranchOf(prev)
			lines := []string{styles.Muted.Render("Worktree:") + "  " + styles.Bold.Render(branch)}
			if params.AdoptIdx >= 0 && selectValue(prev, params.AdoptIdx) == domain.IsolationAdoptValue {
				lines = append(lines, styles.Muted.Render(domain.IsolationAdoptStepName+":")+" "+domain.IsolationAdoptSummary)
			}
			lines = append(lines, "")
			m, ok := stepModel(prev, params.ResolveIdx).(components.EnvResolveModel)
			if body := m.RecapLines(); ok && len(body) > 0 {
				lines = append(lines, body...)
			} else {
				lines = append(lines, "Only safe additions will be applied.")
			}
			lines = append(lines, portRecapLines(params.PortsByBranch[branch])...)
			return components.RecapContent{
				Description: strings.Join(lines, "\n"),
				Actions:     recapActions(params.PortsByBranch[branch]),
			}
		},
	})
}

// recapActions offers the port pass as a choice rather than a fait accompli. A
// worktree with no port to move keeps the single plain confirmation.
func recapActions(plan domain.EnvPortPlan) []components.SelectItem {
	apply := components.SelectItem{Label: domain.EnvApplyActionLabel, Value: applyAction}
	if len(rules.EnvPortRewrites(plan)) == 0 {
		return []components.SelectItem{apply}
	}
	return []components.SelectItem{
		apply,
		{Label: domain.EnvApplyVerbatimLabel, Value: applyVerbatimAction},
	}
}

// portRecapLines announces the [[env_port]] pass that rides along with the apply.
func portRecapLines(plan domain.EnvPortPlan) []string {
	// The recap draws inside the wizard's frame, so the table gets the terminal
	// less what the frame spends on either side of it.
	table := rules.EnvPortTableLines(rules.EnvPortTableParams{
		Plan:  plan,
		Width: recapTableWidth(),
	})
	if len(table) == 0 {
		return nil
	}
	return append([]string{
		"",
		styles.Bold.Render(rules.EnvPortOffsetLabel(plan.Offset)),
	}, table...)
}

// recapTableWidth is what a table has inside the recap's frame, zero when there
// is no terminal to measure — where the table falls back to its own defaults.
func recapTableWidth() int {
	cols := components.TerminalWidth()
	if cols <= 0 {
		return 0
	}
	return cols - domain.RecapFrameChrome
}

// driftBadge renders the per-worktree env-drift pill for the selection list.
func driftBadge(files []domain.EnvFileResult) components.Badge {
	n := actionableCount(files)
	if n == 0 {
		return components.Badge{Text: "in sync", Variant: components.BadgeSuccess, Glyph: domain.BadgeGlyphClean}
	}
	return components.Badge{Text: fmt.Sprintf("%d change(s)", n), Variant: components.BadgeWarning, Glyph: domain.BadgeGlyphDirty}
}

// actionableCount is how many keys need a decision across a worktree's files.
func actionableCount(files []domain.EnvFileResult) int {
	n := 0
	for _, f := range files {
		n += len(rules.EnvKeysWithStatus(rules.EnvDiffFilter{Diff: f.Diff, Status: domain.EnvKeyConflict})) +
			len(rules.EnvKeysWithStatus(rules.EnvDiffFilter{Diff: f.Diff, Status: domain.EnvKeyMissing})) +
			len(rules.EnvKeysWithStatus(rules.EnvDiffFilter{Diff: f.Diff, Status: domain.EnvKeyOrphan}))
	}
	return n
}

func selectValue(steps []components.Step, idx int) string {
	if idx < 0 || idx >= len(steps) {
		return ""
	}
	if sl, ok := steps[idx].Model.(components.SelectListModel); ok {
		return sl.Value()
	}
	return ""
}

func stepModel(steps []components.Step, idx int) any {
	if idx < 0 || idx >= len(steps) {
		return nil
	}
	return steps[idx].Model
}
