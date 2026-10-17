package reparent

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/branch"
)

const (
	KeyBranches = "reparent.branches"
	KeyParent   = "reparent.parent"
	KeyRecap    = "reparent.recap"
)

const confirmReparent = "reparent"

const (
	labelBranches = "Worktrees"
	labelParent   = "New parent"
)

func (f *reparentFlow) session() flow.Session {
	presets := flow.NewAnswers(map[string]string{KeyParent: f.request.To})
	return flow.Session{
		ErrLabel: domain.ReparentWizardErrLabel,
		Presets:  presets.WithValues(KeyBranches, rules.UniqueStrings(f.request.Branches)),
		Steps:    []flow.Step{f.branchesStep(), f.parentStep(), f.recapStep()},
	}
}

func (f *reparentFlow) branchesStep() flow.Step {
	return flow.Step{
		Kind:  flow.StepMultiSelect,
		Key:   KeyBranches,
		Label: labelBranches,
		Build: func(flow.Answers) (flow.StepContent, error) {
			options := f.reparentableOptions()
			if len(options) == 0 {
				return flow.StepContent{}, errors.New(domain.ReparentNoWorktrees)
			}
			return flow.StepContent{
				Title:       domain.ReparentWorktreesTitle,
				Description: domain.ReparentWorktreesDescription,
				Options:     options,
			}, nil
		},
		ValidateSet: func(values []string) error {
			if len(values) == 0 {
				return errors.New(domain.ReparentSelectAtLeastOne)
			}
			return nil
		},
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{}, domain.ErrReparentBranchesRequired
		},
	}
}

// reparentableOptions is every managed worktree: the main one has no recorded
// parent to change, and a worktree's own descendants are excluded later, by the
// parent step, once the selection is known.
func (f *reparentFlow) reparentableOptions() []flow.Option {
	branches := make([]string, 0, len(f.nodes))
	for _, node := range f.nodes {
		if node.IsMain {
			continue
		}
		branches = append(branches, node.Branch)
	}
	sort.Strings(branches)

	options := make([]flow.Option, 0, len(branches))
	for _, name := range branches {
		options = append(options, flow.Option{Label: name, Value: name})
	}
	return options
}

// parentStep offers every branch except the ones the selection rules out, read
// from the answers so a selection made in the picker above narrows it too — not
// only one the request already named.
func (f *reparentFlow) parentStep() flow.Step {
	return flow.Step{
		Kind:     flow.StepBranchSelect,
		Key:      KeyParent,
		Label:    labelParent,
		Pinned:   f.baseBranch(),
		Branches: f.candidates,
		Refresh: func() []domain.BranchCandidate {
			return branch.Refresh(f.runCtx, branch.ListParams{ProjectDir: f.ctx.ProjectDir})
		},
		Build: func(answers flow.Answers) (flow.StepContent, error) {
			selected := answers.Values(KeyBranches)
			return flow.StepContent{
				Title:           domain.ReparentParentTitle,
				Description:     f.parentDescription(selected),
				ExcludeBranches: f.excludedParents(selected),
			}, nil
		},
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{}, domain.ErrReparentParentRequired
		},
		Flag: domain.FlagTo,
	}
}

// parentDescription says what the selection is rebased onto today. One worktree
// names its parent; several only name the count, since they may not share one.
func (f *reparentFlow) parentDescription(selected []string) string {
	if len(selected) != 1 {
		return fmt.Sprintf(domain.ReparentParentBatchFmt, len(selected)) + "\n" + domain.ReparentParentExcluded
	}
	current := f.parentOf(selected[0])
	if current == "" {
		return fmt.Sprintf(domain.ReparentParentNoneFmt, selected[0]) + "\n" + domain.ReparentParentExcluded
	}
	return fmt.Sprintf(domain.ReparentParentSingleFmt, selected[0], current) + "\n" + domain.ReparentParentExcluded
}

func (f *reparentFlow) parentOf(name string) string {
	for _, node := range f.nodes {
		if node.Branch == name {
			return node.SourceBranch
		}
	}
	return ""
}

func (f *reparentFlow) excludedParents(selected []string) []string {
	return rules.ReparentExclusions(rules.ReparentExclusionsParams{
		Nodes:      f.nodes,
		Branches:   selected,
		BaseBranch: f.baseBranch(),
	})
}

func (f *reparentFlow) recapStep() flow.Step {
	return flow.Step{
		Kind:  flow.StepRecap,
		Key:   KeyRecap,
		Label: domain.ReparentRecapLabel,
		Build: func(answers flow.Answers) (flow.StepContent, error) {
			return flow.StepContent{
				Description: f.recap(answers),
				Options:     []flow.Option{{Label: domain.ReparentRecapOption, Value: confirmReparent}},
			}, nil
		},
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{Value: confirmReparent}, nil
		},
	}
}

// recap restates the plan in two fields, reading from Answers so a value that came
// from a flag still shows: --to must never make a line vanish. One worktree names
// the parent it leaves, which is the one thing the new parent does not say; a
// batch has no single one to name, so it lists what moves and where to.
func (f *reparentFlow) recap(answers flow.Answers) string {
	selected := answers.Values(KeyBranches)
	newParent := answers.Value(KeyParent)

	if len(selected) == 1 {
		return domain.ReparentRecapWorktree + selected[0] + "\n" +
			domain.ReparentRecapParent + fmt.Sprintf(domain.ReparentRecapMoveFmt, f.currentParentLabel(selected[0]), newParent)
	}

	return domain.ReparentRecapWorktrees + strings.Join(selected, ", ") + "\n" +
		domain.ReparentRecapNewParent + newParent
}

func (f *reparentFlow) currentParentLabel(name string) string {
	if current := f.parentOf(name); current != "" {
		return current
	}
	return domain.ReparentNoParent
}
