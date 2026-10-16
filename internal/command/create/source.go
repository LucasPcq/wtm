package create

import (
	"slices"

	"github.com/LucasPcq/wtm/internal/domain"
)

func subjects(req Request, f Facts) []string {
	var found []string
	for _, branch := range req.Branches {
		subject := subjectOf(subjectParams{Request: req, Facts: f, Branch: branch})
		if subject != "" && !slices.Contains(found, subject) {
			found = append(found, subject)
		}
	}
	return found
}

type subjectParams struct {
	Request Request
	Facts   Facts
	Branch  string
}

func subjectOf(params subjectParams) string {
	target := params.Facts.target(params.Branch)
	switch {
	case target.State == domain.BranchTargetExisting:
		return params.Branch
	case target.State == domain.BranchTargetNew && target.Upstream == "" && params.Facts.local(params.Request.From):
		return params.Request.From
	}
	return ""
}

type divergedParams struct {
	Request Request
	Facts   Facts
	State   domain.DivergenceState
}

func diverging(params divergedParams) []string {
	return slices.DeleteFunc(subjects(params.Request, params.Facts), func(subject string) bool {
		state, _ := params.Facts.divergence(subject)
		return state != params.State
	})
}

func behind(req Request, f Facts) []string {
	return diverging(divergedParams{Request: req, Facts: f, State: domain.DivergenceBehind})
}

func diverged(req Request, f Facts) []string {
	return diverging(divergedParams{Request: req, Facts: f, State: domain.DivergenceDiverged})
}
