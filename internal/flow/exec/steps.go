package exec

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/shellcmd"
)

const (
	KeySelection = "exec.selection"
	KeyCommand   = "exec.command"
	KeyConfirm   = "exec.confirm"
)

func (f *execFlow) session() flow.Session {
	return flow.Session{
		ErrLabel: domain.ExecWizardErrLabel,
		Presets:  f.presetSelection(),
		Steps:    []flow.Step{f.selectionStep(), f.commandStep(), f.confirmStep()},
	}
}

func (f *execFlow) presetSelection() flow.Answers {
	presets := flow.NewAnswers(map[string]string{KeyCommand: f.params.Request.Command})
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

func (f *execFlow) commandStep() flow.Step {
	return flow.Step{
		Kind:        flow.StepText,
		Key:         KeyCommand,
		Label:       domain.ExecCommandLabel,
		Title:       domain.ExecCommandTitle,
		Description: domain.ExecCommandDescription,
		Validate: func(value string) error {
			if strings.TrimSpace(value) == "" {
				return errors.New(domain.ExecCommandRequired)
			}
			return shellcmd.CheckSyntax(value)
		},
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{}, fmt.Errorf("%w: %w", domain.ErrUsage, domain.ErrExecNoCommand)
		},
	}
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
				Description: f.recap(answers),
				Options:     []flow.Option{{Label: confirmLabel(answers.Values(KeySelection)), Value: domain.ExecConfirmValue}},
			}, nil
		},
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{Value: domain.ExecConfirmValue}, nil
		},
	}
}

func confirmLabel(branches []string) string {
	if len(branches) == 1 {
		return fmt.Sprintf(domain.ExecConfirmOptionFmt, branches[0])
	}
	return fmt.Sprintf(domain.ExecConfirmOptionFmt, rules.ExecWorktreeCount(len(branches)))
}

func (f *execFlow) recap(answers flow.Answers) string {
	return strings.Join([]string{
		domain.ExecRecapWorktrees + strings.Join(answers.Values(KeySelection), ", "),
		domain.ExecRecapCommand + answers.Value(KeyCommand),
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
