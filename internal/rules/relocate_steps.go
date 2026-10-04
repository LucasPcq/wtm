package rules

import "github.com/LucasPcq/wtm/internal/domain"

type RelocateStepStartParams struct {
	Step domain.RelocateStep
	// Parents maps a branch to the parent chosen for its adoption; a missing or
	// empty entry falls back to the step's own, then to BaseBranch.
	Parents    map[string]string
	BaseBranch string
}

// RelocateStepStart is the result a step opens with. A move or an adoption keeps
// its planned status until it is carried out; every other step is already final.
func RelocateStepStart(params RelocateStepStartParams) domain.RelocateStepResult {
	step := params.Step
	res := domain.RelocateStepResult{
		Branch:   step.Branch,
		FromPath: step.FromPath,
		ToPath:   step.ToPath,
		Parent:   relocateParent(params),
		Status:   step.Status,
		Detail:   step.Detail,
	}
	if step.Status == domain.RelocateStatusError {
		res.Detail = domain.RelocateUninspectableDetail
	}
	return res
}

func relocateParent(params RelocateStepStartParams) string {
	if !params.Step.Adopt {
		return ""
	}
	if parent := params.Parents[params.Step.Branch]; parent != "" {
		return parent
	}
	if params.Step.Parent != "" {
		return params.Step.Parent
	}
	return params.BaseBranch
}

func RelocateStepFailed(res domain.RelocateStepResult, err error) domain.RelocateStepResult {
	res.Status = domain.RelocateStatusError
	res.Detail = err.Error()
	return res
}

func RelocateStepDone(step domain.RelocateStep) domain.RelocateStatus {
	switch {
	case step.Status == domain.RelocateStatusAdopt:
		return domain.RelocateStatusAdopted
	case step.Adopt:
		return domain.RelocateStatusMovedAdopted
	default:
		return domain.RelocateStatusMoved
	}
}
