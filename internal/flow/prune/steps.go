package prune

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/orphans"
	"github.com/LucasPcq/wtm/internal/flow/run/owed"
	"github.com/LucasPcq/wtm/internal/rules"
)

const (
	KeySelection = "prune.selection"
	KeyReparent  = "prune.reparent"
	KeyData      = "prune.data"
	KeyConfirm   = "prune.confirm"
)

const (
	confirmYes   = "yes"
	confirmForce = "force"
)

const (
	labelSelection = "Worktrees"
	labelConfirm   = "Confirm"
)

// The reparent decision precedes the recap so the recap is reliably the last,
// unconditional action point.
func (f *pruneFlow) session() flow.Session {
	return flow.Session{
		ErrLabel: domain.PruneWizardErrLabel,
		Presets: flow.NewAnswers(map[string]string{
			KeyReparent: orphans.Preset(f.request.ReparentChildren),
			KeyData:     owed.DataPreset(f.request.DropData),
		}),
		Steps: []flow.Step{
			f.selectionStep(),
			orphans.Step(orphans.StepParams{Key: KeyReparent, Moves: f.orphanPreview}),
			owed.DataStep(owed.DataStepParams{
				Key:      KeyData,
				KeepData: f.request.KeepData,
				Snapshot: func(answers flow.Answers) owed.Snapshot { return f.holdings.Of(f.ctx, answers.Values(KeySelection)) },
			}),
			f.confirmStep(),
		},
	}
}

func (f *pruneFlow) selectionStep() flow.Step {
	return flow.Step{
		Kind:  flow.StepMultiSelect,
		Key:   KeySelection,
		Label: labelSelection,
		Build: func(flow.Answers) (flow.StepContent, error) {
			return flow.StepContent{
				Title:       domain.PruneSelectionTitle,
				Description: domain.MultiSelectHint,
				Options:     f.candidateOptions(),
			}, nil
		},
		// Every match is kept: that is what the filters already selected, and the
		// unsafe ones were only classified in because someone could uncheck them.
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{Values: f.candidateBranches()}, nil
		},
		Summarize: flow.SummarizeSet,
	}
}

// candidateOptions leaves the unsafe ones unchecked, so removing one is always
// something the user opted into.
func (f *pruneFlow) candidateOptions() []flow.Option {
	options := make([]flow.Option, 0, len(f.plan.Selected))
	for _, candidate := range f.plan.Selected {
		tag, tone := candidateTag(candidate)
		options = append(options, flow.Option{
			Label:    candidate.Branch,
			Value:    candidate.Branch,
			Selected: candidate.UnsafeReason == "",
			Tag:      tag,
			Tone:     tone,
		})
	}
	return options
}

func (f *pruneFlow) candidateBranches() []string {
	branches := make([]string, 0, len(f.plan.Selected))
	for _, candidate := range f.plan.Selected {
		branches = append(branches, candidate.Branch)
	}
	return branches
}

func candidateTag(candidate domain.PruneCandidate) (string, domain.Tone) {
	switch candidate.UnsafeReason {
	case domain.PruneSkipDirty:
		return domain.PruneTagDirty, domain.ToneDanger
	case domain.PruneSkipUnpushed:
		return domain.PruneTagUnpushed, domain.ToneDanger
	case domain.PruneSkipOpenPR:
		return domain.PruneTagOpenPR, domain.ToneDanger
	}
	switch candidate.Reason {
	case domain.PruneReasonGone:
		return domain.PruneTagGone, domain.ToneWarning
	case domain.PruneReasonPRMerged:
		return domain.PruneTagMerged, domain.ToneNeutral
	case domain.PruneReasonPRClosed:
		return domain.PruneTagClosed, domain.ToneNeutral
	default:
		return "", domain.ToneNeutral
	}
}

// orphanPreview assumes force so it lists the maximal set of surviving children;
// the confirmed force value narrows it back afterwards.
func (f *pruneFlow) orphanPreview(answers flow.Answers) []domain.ReparentResult {
	return rules.FinalizePrunePlan(rules.FinalizePrunePlanParams{
		Plan:       f.plan,
		Chosen:     answers.Values(KeySelection),
		BaseBranch: f.request.BaseBranch,
		Force:      true,
	}).Reparents
}

func (f *pruneFlow) confirmStep() flow.Step {
	return flow.Step{
		Kind:  flow.StepRecap,
		Key:   KeyConfirm,
		Label: labelConfirm,
		Title: domain.PruneConfirmTitle,
		Build: func(answers flow.Answers) (flow.StepContent, error) {
			selected := answers.Values(KeySelection)
			return flow.StepContent{
				Title:       domain.PruneConfirmTitle,
				Description: f.recap(answers, selected),
				Options:     f.confirmOptions(selected),
				Blockers:    f.blockers(selected),
			}, nil
		},
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{Value: confirmYes}, nil
		},
	}
}

// recap reads the reparent line from the answers, so --reparent-children
// answering the step never makes the line vanish.
func (f *pruneFlow) recap(answers flow.Answers, selected []string) string {
	description := domain.PruneNothingSelected
	if len(selected) > 0 {
		description = fmt.Sprintf(domain.PruneWillPruneFmt, len(selected), strings.Join(selected, ", "))
	}
	data := rules.DataRecapLines(rules.DataRecapLinesParams{
		Held:      f.holdings.Of(f.ctx, selected).Held(),
		StartDown: answers.Value(KeyData) == owed.DataStart,
		KeepData:  f.request.KeepData,
	})
	if len(data) > 0 {
		description += "\n" + strings.Join(data, "\n")
	}
	line := orphans.RecapLine(orphans.RecapLineParams{
		Moves:    f.orphanPreview(answers),
		Reparent: answers.Value(KeyReparent) == orphans.Reparent,
	})
	if line == "" {
		return description
	}
	return description + "\n\n" + line
}

// confirmOptions gates the danger option on a checked unsafe worktree: nothing
// is force-removed by a confirmation that never mentioned it.
func (f *pruneFlow) confirmOptions(selected []string) []flow.Option {
	options := []flow.Option{{Label: domain.PruneConfirmOption, Value: confirmYes}}
	if len(f.unsafeSelected(selected)) == 0 {
		return options
	}
	return append(options,
		flow.Option{Separator: true},
		flow.Option{Label: domain.PruneForceOption, Value: confirmForce, Danger: true},
	)
}

func (f *pruneFlow) blockers(selected []string) []flow.Blocker {
	unsafe := f.unsafeSelected(selected)
	blockers := make([]flow.Blocker, 0, len(unsafe))
	for _, candidate := range unsafe {
		tag, _ := candidateTag(candidate)
		blockers = append(blockers, flow.Blocker{
			Key:   candidate.Branch,
			Label: candidate.Branch + " — " + tag,
		})
	}
	return blockers
}

func (f *pruneFlow) unsafeSelected(selected []string) []domain.PruneCandidate {
	chosen := make(map[string]bool, len(selected))
	for _, branch := range selected {
		chosen[branch] = true
	}
	var unsafe []domain.PruneCandidate
	for _, candidate := range f.plan.Selected {
		if candidate.UnsafeReason != "" && chosen[candidate.Branch] {
			unsafe = append(unsafe, candidate)
		}
	}
	return unsafe
}
