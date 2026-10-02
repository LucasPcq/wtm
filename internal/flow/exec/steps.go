package exec

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
)

const (
	KeySelection = "exec.selection"
	KeyConfirm   = "exec.confirm"
)

func (f *execFlow) session() flow.Session {
	return flow.Session{
		ErrLabel: domain.ExecWizardErrLabel,
		Presets:  f.presetSelection(),
		Steps:    []flow.Step{f.selectionStep(), f.confirmStep()},
	}
}

func (f *execFlow) presetSelection() flow.Answers {
	presets := flow.NewAnswers(nil)
	if f.params.Request.All {
		return presets.WithValues(KeySelection, branchesOf(f.candidates))
	}
	return presets.WithValues(KeySelection, branchesOf(f.selection))
}

func (f *execFlow) selectionStep() flow.Step {
	return flow.Step{
		Kind:  flow.StepMultiSelect,
		Key:   KeySelection,
		Label: domain.ExecSelectionLabel,
		Build: func(flow.Answers) (flow.StepContent, error) {
			return flow.StepContent{
				Title:       domain.ExecSelectionTitle,
				Description: domain.MultiSelectHint,
				Options:     f.selectionOptions(),
			}, nil
		},
		ValidateSet: func(values []string) error {
			if len(values) == 0 {
				return errors.New(domain.ExecSelectAtLeastOne)
			}
			return nil
		},
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{}, fmt.Errorf(domain.ExecSelectionRequiredFmt,
				domain.FlagAll, domain.FlagYes, domain.FlagOutput, domain.OutputJSON)
		},
		Summarize: flow.SummarizeSet,
	}
}

func (f *execFlow) selectionOptions() []flow.Option {
	options := make([]flow.Option, 0, len(f.candidates))
	for _, candidate := range f.candidates {
		label := candidate.Branch
		if candidate.IsMain {
			label += domain.PinnedSuffixBase
		}
		options = append(options, flow.Option{Label: label, Value: candidate.Branch, Selected: candidate.Branch == f.current})
	}
	return options
}

func (f *execFlow) confirmStep() flow.Step {
	return flow.Step{
		Kind:  flow.StepRecap,
		Key:   KeyConfirm,
		Label: domain.ExecConfirmLabel,
		Title: domain.ExecConfirmTitle,
		Build: func(answers flow.Answers) (flow.StepContent, error) {
			return flow.StepContent{
				Title:       domain.ExecConfirmTitle,
				Description: f.recap(answers.Values(KeySelection)),
				Options:     []flow.Option{{Label: domain.ExecConfirmOption, Value: domain.ExecConfirmValue}},
			}, nil
		},
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{Value: domain.ExecConfirmValue}, nil
		},
	}
}

func (f *execFlow) recap(branches []string) string {
	return strings.Join([]string{
		domain.ExecRecapWorktrees + strings.Join(branches, ", "),
		domain.ExecRecapCommand + f.params.Request.Command,
		domain.ExecRecapJobs + strconv.Itoa(f.params.Request.Jobs),
	}, "\n")
}

func branchesOf(worktrees []domain.GitWorktree) []string {
	branches := make([]string, len(worktrees))
	for i, worktree := range worktrees {
		branches[i] = worktree.Branch
	}
	return branches
}
