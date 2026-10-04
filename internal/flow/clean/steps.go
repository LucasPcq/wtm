package clean

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/orphans"
	"github.com/LucasPcq/wtm/internal/flow/run/owed"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

const (
	KeyWorktree = "clean.worktree"
	KeyReparent = "clean.reparent"
	KeyData     = "clean.data"
	KeyDelete   = "clean.delete"
)

const (
	deleteYes   = "yes"
	deleteSafe  = "safe"
	deleteForce = "force"
)

const (
	labelWorktree = "Worktrees"
	labelDelete   = "Delete"
)

// The order makes the delete confirmation the last, unconditional action point.
func (f *cleanFlow) session() flow.Session {
	return flow.Session{
		ErrLabel: domain.CleanWizardErrLabel,
		Presets: flow.NewAnswers(map[string]string{
			KeyReparent: orphans.Preset(f.request.ReparentChildren),
			KeyData:     owed.DataPreset(f.request.DropData),
		}).WithValues(KeyWorktree, f.request.Branches),
		Steps: []flow.Step{
			f.worktreeStep(),
			orphans.Step(orphans.StepParams{
				Key:   KeyReparent,
				Moves: func(answers flow.Answers) []domain.ReparentResult { return f.reparents(answers.Values(KeyWorktree)) },
			}),
			owed.DataStep(owed.DataStepParams{
				Key:      KeyData,
				KeepData: f.request.KeepData,
				Snapshot: func(answers flow.Answers) owed.Snapshot { return f.holdings.Of(f.ctx, answers.Values(KeyWorktree)) },
			}),
			f.deleteStep(),
		},
	}
}

func (f *cleanFlow) worktreeStep() flow.Step {
	return flow.Step{
		Kind:        flow.StepMultiSelect,
		Key:         KeyWorktree,
		Label:       labelWorktree,
		Title:       domain.CleanPickerTitle,
		Description: domain.MultiSelectHint,
		Build: func(flow.Answers) (flow.StepContent, error) {
			options, err := f.cleanableOptions()
			if err != nil {
				return flow.StepContent{}, err
			}
			return flow.StepContent{Title: domain.CleanPickerTitle, Description: domain.MultiSelectHint, Options: options}, nil
		},
		ValidateSet: func(values []string) error {
			if len(values) == 0 {
				return errors.New(domain.CleanSelectionRequired)
			}
			return nil
		},
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{}, domain.ErrCleanBranchRequired
		},
		Summarize: flow.SummarizeSet,
		Arg:       true,
	}
}

func (f *cleanFlow) cleanableOptions() ([]flow.Option, error) {
	worktrees, err := worktree.ListAll(worktree.ListAllParams{ProjectDir: f.ctx.ProjectDir})
	if err != nil {
		return nil, fmt.Errorf("list worktrees: %w", err)
	}
	options := make([]flow.Option, 0, len(worktrees))
	for _, wt := range worktrees {
		if wt.IsMain {
			continue
		}
		options = append(options, flow.Option{Label: wt.Branch, Value: wt.Branch})
	}
	sort.Slice(options, func(i, j int) bool { return options[i].Value < options[j].Value })
	if len(options) == 0 {
		return nil, errors.New(domain.CleanNothingToClean)
	}
	return options, nil
}

func (f *cleanFlow) deleteStep() flow.Step {
	content := func(answers flow.Answers) (flow.StepContent, error) {
		selected := answers.Values(KeyWorktree)
		checks, err := f.checksOf(selected)
		if err != nil {
			return flow.StepContent{}, err
		}
		return flow.StepContent{
			Title: domain.CleanDeleteTitle,
			Description: deleteRecap(deleteRecapParams{
				Checks:   checks,
				Reparent: f.reparentLine(answers),
				Namespaces: rules.DataRecapLines(rules.DataRecapLinesParams{
					Held:      f.holdings.Of(f.ctx, selected).Held(),
					StartDown: answers.Value(KeyData) == owed.DataStart,
					KeepData:  f.request.KeepData,
				}),
			}),
			Options:  deleteOptions(checks),
			Blockers: blockersOf(checks),
		}, nil
	}

	step := flow.Step{
		Kind:           flow.StepRecap,
		Key:            KeyDelete,
		Label:          labelDelete,
		Title:          domain.CleanDeleteTitle,
		LoadingMessage: domain.CleanCheckLoading,
		Resolve:        f.resolveDelete,
	}
	// Named worktrees were already checked, so their recap needs no I/O. Picked
	// ones are checked then and there, over the network.
	if len(f.request.Branches) > 0 {
		step.Build = content
		return step
	}
	step.Load = content
	return step
}

// resolveDelete keeps the safety check while answering: --yes is the
// confirmation axis, not a licence to remove something unsafe, and the user
// named every worktree, so none is left out behind their back. Only --force
// lifts the refusal.
func (f *cleanFlow) resolveDelete(answers flow.Answers) (flow.Answer, error) {
	if f.request.Force {
		return flow.Answer{Value: deleteYes}, nil
	}
	selected := answers.Values(KeyWorktree)
	if err := f.checkAll(selected); err != nil {
		return flow.Answer{}, err
	}
	// Unreadable is left to the removal, which reports it per worktree.
	checks, _ := f.checksOf(selected)
	if refusal, unsafe := rules.CleanUnsafeRefusal(checks); unsafe {
		return flow.Answer{}, errors.New(refusal)
	}
	return flow.Answer{Value: deleteYes}, nil
}

// deleteOptions never offers a plain removal of an unsafe batch: git would
// refuse some halfway, after their worktree was gone. One worktree keeps the
// options it always had, unless it is locked: git refuses that one outright.
func deleteOptions(checks []domain.CleanCheckResult) []flow.Option {
	unsafe := 0
	for _, check := range checks {
		if rules.HasWarnings(check) {
			unsafe++
		}
	}
	plain := flow.Option{Label: domain.CleanDeleteOption, Value: deleteYes}
	if unsafe == 0 {
		return []flow.Option{plain}
	}
	force := flow.Option{Label: domain.CleanForceDeleteOption, Value: deleteForce, Danger: true}
	if len(checks) == 1 && checks[0].IsLocked {
		return []flow.Option{force}
	}
	if len(checks) == 1 {
		return []flow.Option{plain, {Separator: true}, force}
	}
	force.Label = domain.CleanForceDeleteManyOption
	if unsafe == len(checks) {
		return []flow.Option{force}
	}
	safe := flow.Option{Label: fmt.Sprintf(domain.CleanDeleteSafeOptionFmt, len(checks)-unsafe), Value: deleteSafe}
	return []flow.Option{safe, {Separator: true}, force}
}

func blockersOf(checks []domain.CleanCheckResult) []flow.Blocker {
	refusals := rules.CleanBatchBlockers(checks)
	blockers := make([]flow.Blocker, 0, len(refusals))
	for _, refusal := range refusals {
		blockers = append(blockers, flow.Blocker{Key: refusal.Key, Label: refusal.Label})
	}
	return blockers
}

type deleteRecapParams struct {
	Checks   []domain.CleanCheckResult
	Reparent string
	// Namespaces are the lines naming the data this clean gives back, or the one
	// saying it is kept. A flag must never make a line disappear from a recap,
	// and this one carries a DROP DATABASE.
	Namespaces []string
}

func deleteRecap(params deleteRecapParams) string {
	var lines []string
	for _, blocker := range rules.CleanBatchBlockers(params.Checks) {
		lines = append(lines, blocker.Label)
	}
	if len(lines) > 0 {
		lines = append(lines, "")
	}
	lines = append(lines, targetLines(params.Checks)...)
	lines = append(lines, params.Namespaces...)
	if params.Reparent != "" {
		lines = append(lines, "", params.Reparent)
	}
	return strings.Join(lines, "\n")
}

func targetLines(checks []domain.CleanCheckResult) []string {
	if len(checks) <= 1 {
		var check domain.CleanCheckResult
		if len(checks) == 1 {
			check = checks[0]
		}
		return []string{
			domain.CleanWillDelete,
			domain.CleanWillDeleteWorktree + check.WorktreePath,
			domain.CleanWillDeleteBranch + check.Branch,
		}
	}
	lines := []string{fmt.Sprintf(domain.CleanWillDeleteManyFmt, len(checks))}
	for _, check := range checks {
		lines = append(lines, fmt.Sprintf(domain.CleanWillDeleteRowFmt, check.Branch, check.WorktreePath))
	}
	return lines
}

func (f *cleanFlow) reparentLine(answers flow.Answers) string {
	return orphans.RecapLine(orphans.RecapLineParams{
		Moves:    f.reparents(answers.Values(KeyWorktree)),
		Reparent: answers.Value(KeyReparent) == orphans.Reparent,
	})
}
