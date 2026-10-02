package clean

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
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
	reparentYes = "reparent"
	reparentNo  = "orphan"
	deleteYes   = "yes"
	deleteSafe  = "safe"
	deleteForce = "force"
)

const (
	labelWorktree = "Worktrees"
	labelReparent = "Reparent children"
	labelDelete   = "Delete"
)

// The order makes the delete confirmation the last, unconditional action point.
func (f *cleanFlow) session() flow.Session {
	return flow.Session{
		ErrLabel: domain.CleanWizardErrLabel,
		Presets: flow.NewAnswers(map[string]string{
			KeyReparent: f.presetReparent(),
			KeyData:     owed.DataPreset(f.request.DropData),
		}).WithValues(KeyWorktree, f.request.Branches),
		Steps: []flow.Step{
			f.worktreeStep(),
			f.reparentStep(),
			owed.DataStep(owed.DataStepParams{
				Key:      KeyData,
				KeepData: f.request.KeepData,
				Snapshot: func(answers flow.Answers) owed.Snapshot { return f.holdings(answers.Values(KeyWorktree)) },
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

func (f *cleanFlow) reparentStep() flow.Step {
	return flow.Step{
		Kind:  flow.StepSelect,
		Key:   KeyReparent,
		Label: labelReparent,
		Skip: func(answers flow.Answers) (bool, string) {
			if len(f.reparents(answers.Values(KeyWorktree))) == 0 {
				return true, domain.CleanNoOrphanedChildren
			}
			return false, ""
		},
		Build: func(answers flow.Answers) (flow.StepContent, error) {
			moves := f.reparents(answers.Values(KeyWorktree))
			return flow.StepContent{
				Description: reparentProposal(moves),
				Options: []flow.Option{
					{Label: reparentOptionLabel(moves), Value: reparentYes},
					{Separator: true},
					{Label: domain.CleanOrphanOption, Value: reparentNo},
				},
			}, nil
		},
		// Reparenting is opt-in: `wtm reparent` can still move the children afterwards.
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{Value: reparentNo}, nil
		},
		Summarize: func(answer flow.Answer) string {
			if answer.Value == reparentYes {
				return domain.CleanReparentSummary
			}
			return domain.CleanOrphanSummary
		},
		Flag: domain.FlagReparentChildren,
	}
}

func (f *cleanFlow) presetReparent() string {
	if f.request.ReparentChildren {
		return reparentYes
	}
	return ""
}

func reparentProposal(moves []domain.ReparentResult) string {
	lines := make([]string, 0, len(moves)+1)
	lines = append(lines, domain.CleanReparentIntro)
	for _, move := range moves {
		lines = append(lines, fmt.Sprintf(domain.CleanReparentChildFmt, move.Branch, move.NewParent, move.OldParent))
	}
	return strings.Join(lines, "\n")
}

// One destination reads as it always did; several are named one by one in the
// proposal above the options.
func reparentOptionLabel(moves []domain.ReparentResult) string {
	if parent, single := soleParent(moves); single {
		return fmt.Sprintf(domain.CleanReparentOptionFmt, parent, len(moves))
	}
	return fmt.Sprintf(domain.CleanReparentManyOptionFmt, len(moves))
}

func soleParent(moves []domain.ReparentResult) (string, bool) {
	if len(moves) == 0 {
		return "", false
	}
	for _, move := range moves[1:] {
		if move.NewParent != moves[0].NewParent {
			return "", false
		}
	}
	return moves[0].NewParent, true
}

func (f *cleanFlow) deleteStep() flow.Step {
	content := func(answers flow.Answers) (flow.StepContent, error) {
		selected := answers.Values(KeyWorktree)
		checks := f.checksOf(selected)
		return flow.StepContent{
			Title: domain.CleanDeleteTitle,
			Description: deleteRecap(deleteRecapParams{
				Checks:   checks,
				Reparent: f.reparentLine(answers),
				Namespaces: rules.DataRecapLines(rules.DataRecapLinesParams{
					Held:      f.holdings(selected).Held(),
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
	if refusal, unsafe := rules.CleanUnsafeRefusal(f.checksOf(selected)); unsafe {
		return flow.Answer{}, errors.New(refusal)
	}
	return flow.Answer{Value: deleteYes}, nil
}

// deleteOptions never offers a plain removal of an unsafe batch: git would
// refuse some halfway, after their worktree was gone. One worktree keeps the
// options it always had.
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

// holdings is read once per selection: the data step and the recap both need
// it, and it asks the daemon which services are up.
func (f *cleanFlow) holdings(selected []string) owed.Snapshot {
	if len(selected) == 0 {
		return owed.Snapshot{}
	}
	key := strings.Join(selected, "\x00")
	if snapshot, cached := f.snapshots[key]; cached {
		return snapshot
	}
	if f.snapshots == nil {
		f.snapshots = map[string]owed.Snapshot{}
	}
	snapshot := owed.Read(owed.ReadParams{Context: f.ctx, Branches: selected})
	f.snapshots[key] = snapshot
	return snapshot
}

func (f *cleanFlow) reparentLine(answers flow.Answers) string {
	moves := f.reparents(answers.Values(KeyWorktree))
	if len(moves) == 0 {
		return ""
	}
	if answers.Value(KeyReparent) != reparentYes {
		return fmt.Sprintf(domain.CleanRecapOrphanFmt, len(moves))
	}
	if parent, single := soleParent(moves); single {
		return fmt.Sprintf(domain.CleanRecapReparentFmt, len(moves), parent)
	}
	return fmt.Sprintf(domain.CleanRecapReparentManyFmt, len(moves))
}
