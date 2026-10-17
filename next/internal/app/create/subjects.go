package create

import (
	"slices"

	"github.com/LucasPcq/wtm/next/internal/wtm/branch"
)

func (f Facts) checksOut(req Request) bool {
	return slices.ContainsFunc(f.decisions(req), func(d decision) bool {
		planned, isPlanned := d.(BranchPlan)
		return isPlanned && planned.Action == ActionCheckOut
	})
}

func (f Facts) subjectsAt(req Request, standing branch.Divergence) []string {
	var subjects []string
	for _, d := range f.decisions(req) {
		planned, isPlanned := d.(BranchPlan)
		subject := ""
		switch {
		case !isPlanned:
		case planned.Action == ActionCheckOut:
			subject = planned.Branch
		case planned.Action == ActionCreate && f.local(req.From):
			subject = req.From
		}
		if subject != "" && f.standing(subject) == standing && !slices.Contains(subjects, subject) {
			subjects = append(subjects, subject)
		}
	}
	return subjects
}
