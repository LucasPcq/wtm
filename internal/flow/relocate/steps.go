package relocate

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/decide"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/branch"
)

const (
	KeyBasePathGate = "relocate.base_path_gate"
	KeyBasePath     = "relocate.base_path"
	KeyRecap        = "relocate.recap"
	keyParentPrefix = "relocate.parent."
)

const (
	basePathKeep   = "keep"
	basePathChange = "change"
	confirmApply   = "apply"
)

func KeyParent(branch string) string { return keyParentPrefix + branch }

func (f *relocateFlow) session() flow.Session {
	presets := map[string]string{}
	if f.request.To != "" {
		presets[KeyBasePathGate] = basePathChange
		presets[KeyBasePath] = f.request.To
	}

	steps := []flow.Step{f.gateStep(), f.basePathStep()}
	adoptions := rules.PlanAdoptions(f.plan)
	candidates := f.candidates(len(adoptions) > 0)
	for index, adoption := range adoptions {
		steps = append(steps, f.parentStep(parentStepParams{
			Adoption:   adoption,
			Candidates: candidates,
			// Every picker shares one list, so refreshing it once feeds them all.
			Refresh: index == 0,
		}))
	}
	steps = append(steps, f.recapStep())

	return flow.Session{
		ErrLabel: domain.RelocateWizardErrLabel,
		Presets:  flow.NewAnswers(presets),
		Steps:    steps,
	}
}

func (f *relocateFlow) gateStep() flow.Step {
	current := f.configBasePath()
	return flow.Step{
		Kind:        flow.StepSelect,
		Key:         KeyBasePathGate,
		Label:       domain.RelocateBasePathGateLabel,
		Title:       domain.RelocateBasePathGateTitle,
		Description: fmt.Sprintf(domain.RelocateBasePathGateDescFmt, current),
		Options: []flow.Option{
			{Label: fmt.Sprintf(domain.RelocateBasePathKeepFmt, current), Value: basePathKeep},
			{Label: domain.RelocateBasePathChange, Value: basePathChange},
		},
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{Value: basePathKeep}, nil
		},
	}
}

func (f *relocateFlow) basePathStep() flow.Step {
	current := f.configBasePath()
	return flow.Step{
		Kind:        flow.StepText,
		Key:         KeyBasePath,
		Label:       domain.RelocateBasePathValueLabel,
		Title:       domain.RelocateBasePathValueLabel,
		Description: domain.RelocateBasePathValueDesc,
		Default:     current,
		Validate:    rules.ValidateBasePathEntry,
		Skip: func(answers flow.Answers) (bool, string) {
			return answers.Value(KeyBasePathGate) != basePathChange, ""
		},
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{Value: current}, nil
		},
	}
}

// candidates lists the parents only when a picker will show them: an unattended
// run has nothing to pick, and listing costs a few git calls.
func (f *relocateFlow) candidates(needed bool) []domain.BranchCandidate {
	if !needed || !f.prompter.Interactive() {
		return nil
	}
	return decide.BranchCandidates(f.runCtx, f.ctx.ProjectDir)
}

type parentStepParams struct {
	Adoption   domain.RelocateStep
	Candidates []domain.BranchCandidate
	Refresh    bool
}

func (f *relocateFlow) parentStep(params parentStepParams) flow.Step {
	name := params.Adoption.Branch
	label := fmt.Sprintf(domain.RelocateParentLabelFmt, name)
	step := flow.Step{
		Kind:     flow.StepBranchSelect,
		Key:      KeyParent(name),
		Label:    label,
		Pinned:   f.request.BaseBranch,
		Branches: params.Candidates,
		Build: func(flow.Answers) (flow.StepContent, error) {
			return flow.StepContent{
				Title:           label,
				Description:     fmt.Sprintf(domain.RelocateParentDescFmt, name),
				ExcludeBranches: []string{name},
			}, nil
		},
		// --yes adopts onto the base branch, which `wtm reparent` can change later.
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{Value: rules.RelocateStepStart(rules.RelocateStepStartParams{
				Step:       params.Adoption,
				BaseBranch: f.request.BaseBranch,
			}).Parent}, nil
		},
	}
	if params.Refresh {
		step.Refresh = func() []domain.BranchCandidate {
			return branch.Refresh(f.runCtx, branch.ListParams{ProjectDir: f.ctx.ProjectDir})
		}
	}
	return step
}

// recapStep is skipped rather than asked when nothing would change, so no one is
// asked to confirm a no-op.
func (f *relocateFlow) recapStep() flow.Step {
	return flow.Step{
		Kind:  flow.StepRecap,
		Key:   KeyRecap,
		Label: domain.RelocateApplyLabel,
		Skip: func(answers flow.Answers) (bool, string) {
			return !f.changes(f.basePath(answers)), ""
		},
		Build: func(answers flow.Answers) (flow.StepContent, error) {
			return flow.StepContent{
				Description: f.recap(answers),
				Options:     []flow.Option{{Label: domain.RelocateApplyOption, Value: confirmApply}},
			}, nil
		},
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{Value: confirmApply}, nil
		},
	}
}

// recap projects the plan onto the base_path chosen, and names the change of
// base_path even when --to made it: a flag never erases a recap line.
func (f *relocateFlow) recap(answers flow.Answers) string {
	basePath := f.basePath(answers)
	plan := f.plan
	if basePath != f.targetBasePath() {
		plan = rules.ReprojectRelocatePlan(rules.ReprojectRelocatePlanParams{
			Plan:       f.plan,
			ProjectDir: f.ctx.ProjectDir,
			BasePath:   basePath,
		})
	}
	return rules.RelocateRecap(rules.RelocateRecapParams{
		Plan:             plan,
		Parents:          f.parents(answers),
		PreviousBasePath: f.configBasePath(),
	})
}

func (f *relocateFlow) basePath(answers flow.Answers) string {
	if answers.Value(KeyBasePathGate) != basePathChange {
		return f.configBasePath()
	}
	if value := answers.Value(KeyBasePath); value != "" {
		return value
	}
	return f.configBasePath()
}

func (f *relocateFlow) parents(answers flow.Answers) map[string]string {
	adoptions := rules.PlanAdoptions(f.plan)
	parents := make(map[string]string, len(adoptions))
	for _, adoption := range adoptions {
		if value := answers.Value(KeyParent(adoption.Branch)); value != "" {
			parents[adoption.Branch] = value
		}
	}
	return parents
}
