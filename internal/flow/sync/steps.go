package sync

import (
	"errors"
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

const (
	KeySelection = "sync.selection"
	KeyConflict  = "sync.conflict"
	KeyParents   = "sync.parents"
	KeyConfirm   = "sync.confirm"
)

const (
	conflictNormal = "normal"
	conflictKeep   = "keep"
	parentFF       = "fast-forward"
	parentKeep     = "keep"
	confirmSync    = "sync"
)

const (
	labelSelection = "Worktrees"
	labelConflict  = "On conflict"
	labelParents   = "Parent branches"
	labelConfirm   = "Confirm"
)

func (f *syncFlow) session() flow.Session {
	presets := flow.NewAnswers(map[string]string{KeyParents: f.presetParents()})
	return flow.Session{
		ErrLabel: domain.SyncWizardErrLabel,
		Presets:  f.presetSelection(presets),
		Steps:    []flow.Step{f.selectionStep(), f.conflictStep(), f.parentsStep(), f.confirmStep()},
	}
}

// presetSelection settles the selection when args or --all already did. --all
// presets even when it names nothing — a stack reduced to its base still has a
// base to refresh, and an unset preset would open the picker the flag exists to
// answer. The preset only previews the resolved list; the service still receives
// nil, which is what "every worktree" means to it (see rules.SyncIncludesBase).
func (f *syncFlow) presetSelection(presets flow.Answers) flow.Answers {
	if f.request.All {
		return presets.With(KeySelection, flow.Answer{Values: rules.SyncAllBranches(f.statuses)})
	}
	return presets.WithValues(KeySelection, f.selection)
}

func (f *syncFlow) selectionStep() flow.Step {
	return flow.Step{
		Kind:  flow.StepMultiSelect,
		Key:   KeySelection,
		Label: labelSelection,
		Build: func(flow.Answers) (flow.StepContent, error) {
			return flow.StepContent{
				Title:       domain.SyncSelectionTitle,
				Description: domain.MultiSelectHint,
				Options:     f.selectionOptions(),
			}, nil
		},
		ValidateSet: func(values []string) error {
			if len(values) == 0 {
				return errors.New(domain.SyncSelectAtLeastOne)
			}
			return nil
		},
		Resolve: func(flow.Answers) (flow.Answer, error) {
			// --all answers the step even when it lists nothing: the preset only
			// carries the branches there are to preview, and a stack reduced to its
			// base still has a base to refresh.
			if f.request.All {
				return flow.Answer{}, nil
			}
			return flow.Answer{}, fmt.Errorf(domain.SyncSelectionRequiredFmt,
				domain.FlagAll, domain.FlagYes, domain.FlagOutput, domain.OutputJSON)
		},
		Summarize: flow.SummarizeSet,
	}
}

func (f *syncFlow) selectionOptions() []flow.Option {
	prechecked := make(map[string]bool, len(f.request.Precheck))
	for _, branch := range f.request.Precheck {
		prechecked[branch] = true
	}

	options := make([]flow.Option, 0, len(f.statuses))
	for _, status := range f.statuses {
		tag, tone := statusTag(status)
		options = append(options, flow.Option{
			Label:    statusLabel(status),
			Value:    status.Branch,
			Selected: prechecked[status.Branch],
			Tag:      tag,
			Tone:     tone,
		})
	}
	return options
}

// statusLabel tells the base apart from the worktrees that hang off it.
func statusLabel(status domain.WorktreeStatus) string {
	if status.IsParent {
		return status.Branch + domain.PinnedSuffixBase
	}
	return status.Branch
}

// statusTag names what the cascade would skip, so a worktree left out is left
// out for a reason the user can read.
func statusTag(status domain.WorktreeStatus) (string, domain.Tone) {
	switch {
	case status.IsParent:
		return "", domain.ToneNeutral
	case status.RebaseInProgress:
		return domain.SyncTagRebasing, domain.ToneWarning
	case status.IsDirty:
		return domain.SyncTagDirty, domain.ToneWarning
	}
	return "", domain.ToneNeutral
}

// conflictStep is skipped when nothing is rebased: a base-only refresh has no
// conflict to have an opinion about.
func (f *syncFlow) conflictStep() flow.Step {
	return flow.Step{
		Kind:  flow.StepSelect,
		Key:   KeyConflict,
		Label: labelConflict,
		Skip: func(answers flow.Answers) (bool, string) {
			if f.request.DryRun {
				return true, domain.SyncDryRunNoQuestion
			}
			if f.nothingToRebase(answers) {
				return true, domain.SyncNoRebaseStep
			}
			return false, ""
		},
		Build: func(answers flow.Answers) (flow.StepContent, error) {
			return flow.StepContent{
				Title:       domain.SyncConflictTitle,
				Description: conflictDescription(len(answers.Values(KeySelection))),
				Options:     f.conflictOptions(),
			}, nil
		},
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{Value: f.conflictDefault()}, nil
		},
		Summarize: func(answer flow.Answer) string {
			if answer.Value == conflictKeep {
				return domain.SyncConflictKeepSummary
			}
			return domain.SyncConflictNormalSummary
		},
		Flag: domain.FlagKeepConflict,
	}
}

// nothingToRebase decides from the selection alone — no git call from a Skip. An
// --all that covers nothing reads back as no selection at all, which everywhere
// else means "every worktree", so it is answered from the worktrees --all would
// have named rather than from what came back.
func (f *syncFlow) nothingToRebase(answers flow.Answers) bool {
	if f.request.All {
		return len(rules.SyncAllBranches(f.statuses)) == 0
	}
	return rules.SyncSelectsOnlyBase(rules.SyncIncludesBaseParams{
		Selected:   answers.Values(KeySelection),
		BaseBranch: f.request.BaseBranch,
	})
}

func conflictDescription(count int) string {
	if count == 0 {
		return domain.SyncConflictIntro
	}
	return fmt.Sprintf(domain.SyncCounterFmt, count) + "\n\n" + domain.SyncConflictIntro
}

// conflictOptions leads with what --keep-conflict asked for rather than
// answering the step: the question stays visible, and its other outcome stays
// one keystroke away.
func (f *syncFlow) conflictOptions() []flow.Option {
	normal := flow.Option{Label: domain.SyncConflictNormal, Value: conflictNormal}
	keep := flow.Option{Label: domain.SyncConflictKeep, Value: conflictKeep, Danger: true}
	if f.request.KeepConflict {
		return []flow.Option{keep, normal}
	}
	return []flow.Option{normal, keep}
}

func (f *syncFlow) conflictDefault() string {
	if f.request.KeepConflict {
		return conflictKeep
	}
	return conflictNormal
}

// parentsStep asks about the parents no step covers. It is skipped when the
// selection leaves none of them behind their remote.
func (f *syncFlow) parentsStep() flow.Step {
	return flow.Step{
		Kind:  flow.StepSelect,
		Key:   KeyParents,
		Label: labelParents,
		Skip: func(answers flow.Answers) (bool, string) {
			if len(f.staleParents(answers)) == 0 {
				return true, domain.SyncNoStaleParent
			}
			return false, ""
		},
		Build: func(answers flow.Answers) (flow.StepContent, error) {
			return flow.StepContent{
				Title:       domain.SyncParentsTitle,
				Description: parentLines(f.staleParents(answers)) + "\n\n" + domain.SyncParentDescription,
				Options: []flow.Option{
					{Label: domain.SyncParentFFOption, Value: parentFF},
					{Label: domain.SyncParentKeepOption, Value: parentKeep},
				},
			}, nil
		},
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{Value: parentKeep}, nil
		},
		Summarize: func(answer flow.Answer) string {
			if answer.Value == parentFF {
				return domain.SyncParentFFSummary
			}
			return domain.SyncParentKeepSummary
		},
		Flag: domain.FlagFFParents,
		PresetFlag: func(answer flow.Answer) string {
			if answer.Value == parentFF {
				return domain.FlagFFParents
			}
			return domain.FlagNoFFParents
		},
	}
}

func parentLines(parents []domain.ParentUpdate) string {
	lines := make([]string, 0, len(parents))
	for _, parent := range parents {
		lines = append(lines, fmt.Sprintf(domain.SyncParentLineFmt,
			parent.Branch, rules.CommitCountLabel(parent.Behind),
			domain.RemoteBranchPrefix, parent.Branch,
			strings.Join(parent.Children, ", ")))
	}
	return strings.Join(lines, "\n")
}

// presetParents settles the question from the flags, through the same rule the
// command used to call. The step stays listed so a flag never makes a recap line
// disappear.
func (f *syncFlow) presetParents() string {
	switch rules.ParentFlagsDecision(rules.DecideParentFastForwardParams{
		FF:          f.request.FFParents,
		NoFF:        f.request.NoFFParents,
		Interactive: f.prompter.Interactive(),
	}) {
	case rules.ParentFastForward:
		return parentFF
	case rules.ParentAsk:
		return ""
	default:
		return parentKeep
	}
}

// confirmStep previews the cascade. Building the plan walks every selected
// worktree's history, so it goes through Load — off the UI goroutine, behind a
// spinner. A preview never puts it: --dry-run confirms nothing, and the plan
// then reaches the surface through Planned instead.
func (f *syncFlow) confirmStep() flow.Step {
	return flow.Step{
		Kind:           flow.StepRecap,
		Key:            KeyConfirm,
		Label:          labelConfirm,
		Title:          domain.SyncConfirmTitle,
		LoadingMessage: domain.SyncPlanComputing,
		Skip: func(flow.Answers) (bool, string) {
			if f.request.DryRun {
				return true, domain.SyncDryRunNoQuestion
			}
			return false, ""
		},
		Load: func(answers flow.Answers) (flow.StepContent, error) {
			plan, err := f.planFor(answers)
			if err != nil {
				return flow.StepContent{}, err
			}
			return f.confirmContent(plan, answers), nil
		},
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{Value: confirmSync}, nil
		},
	}
}

func (f *syncFlow) confirmContent(plan domain.SyncPlan, answers flow.Answers) flow.StepContent {
	return flow.StepContent{
		Title: domain.SyncConfirmTitle,
		Description: confirmDescription(confirmDescriptionParams{
			Plan:         plan,
			KeepConflict: answers.Value(KeyConflict) == conflictKeep,
		}),
		Options: []flow.Option{{Label: domain.SyncConfirmOption, Value: confirmSync}},
	}
}

type confirmDescriptionParams struct {
	Plan         domain.SyncPlan
	KeepConflict bool
}

func confirmDescription(params confirmDescriptionParams) string {
	description := confirmQuestion(params.Plan)
	if text := rules.SprintSyncPlan(params.Plan); text != "" {
		description = text + "\n\n" + description
	}
	if params.KeepConflict {
		description += "\n\n" + domain.SyncKeepConflictWarning
	}
	return description
}

// confirmQuestion asks about what the run will actually do. A selection holding
// only the root rebases nothing, so counting worktrees there would announce a
// cascade that never comes and name a parent nothing hangs off.
func confirmQuestion(plan domain.SyncPlan) string {
	if len(plan.Steps) > 0 {
		return fmt.Sprintf(domain.SyncConfirmPrompt, len(plan.Steps))
	}
	return fmt.Sprintf(domain.SyncConfirmBaseFmt, plan.BaseBranch)
}

type syncParamsInput struct {
	Selected     []string
	KeepConflict bool
	FastForward  bool
}

// syncParams is the one place the request becomes service inputs. --all keeps its
// meaning by passing nil: the service reads an empty selection as "every worktree".
func (f *syncFlow) syncParams(input syncParamsInput) worktree.SyncParams {
	selected := input.Selected
	if f.request.All {
		selected = nil
	}
	return worktree.SyncParams{
		ProjectDir:       f.ctx.ProjectDir,
		StateDir:         f.ctx.StateDir,
		Config:           f.ctx.Config,
		BaseBranch:       f.request.BaseBranch,
		DryRun:           f.request.DryRun,
		KeepConflict:     input.KeepConflict,
		SelectedBranches: selected,
		// Dry-run stays offline, so it never refreshes a parent whatever was asked.
		FastForwardParents: input.FastForward && !f.request.DryRun,
	}
}

// planFor previews the cascade for the selection as it stands. Conflict mode does
// not affect the plan, so it is not read here. A failure is returned rather than
// folded into an empty plan, which would read as "nothing to rebase".
func (f *syncFlow) planFor(answers flow.Answers) (domain.SyncPlan, error) {
	plan, err := worktree.PlanSync(f.syncParams(syncParamsInput{Selected: answers.Values(KeySelection)}))
	if err != nil {
		//lint:ignore ST1005 domain.SyncPlanFailedFmt reproduces the old picker's
		//text verbatim; a lowercase rewrite would change observable output.
		return domain.SyncPlan{}, fmt.Errorf(domain.SyncPlanFailedFmt, err)
	}
	return plan, nil
}

// staleParents narrows the inspection to the current selection. The scan itself
// ran once, before the session; nil means the run never inspected them. The
// narrowing replans the cascade, and the step reads it from both Skip and Build
// on every rebuild — hence the memo, keyed by the selection it answers for.
func (f *syncFlow) staleParents(answers flow.Answers) []domain.ParentUpdate {
	if len(f.classified) == 0 {
		return nil
	}
	branches := answers.Values(KeySelection)
	key := strings.Join(branches, "\n")
	if cached, known := f.stale[key]; known {
		return cached
	}
	parents := worktree.StaleParents(worktree.StaleParentsParams{
		Sync:       f.syncParams(syncParamsInput{Selected: branches}),
		Branches:   branches,
		Classified: f.classified,
	})
	if f.stale == nil {
		f.stale = map[string][]domain.ParentUpdate{}
	}
	f.stale[key] = parents
	return parents
}
