package create

import (
	"slices"
	"strconv"

	"github.com/LucasPcq/wtm/next/internal/wtm/env"
	"github.com/LucasPcq/wtm/next/internal/wtm/worktree"

	"github.com/LucasPcq/wtm/next/internal/wtm"

	"github.com/LucasPcq/wtm/next/internal/wtm/project"
)

type Action string

const (
	ActionCreate    Action = "create"
	ActionCheckOut  Action = "check_out"
	ActionResume    Action = "resume"
	ActionUnchanged Action = "unchanged"
)

type Planned struct {
	Branches    []BranchPlan        `json:"branches"`
	From        string              `json:"from,omitempty"`
	EnvFrom     project.EnvStrategy `json:"env_from"`
	Isolation   project.Isolation   `json:"isolation,omitempty"`
	FastForward []string            `json:"fast_forward,omitempty"`
	Remember    []string            `json:"remember,omitempty"`
}

type BranchPlan struct {
	Branch   string `json:"branch"`
	Path     string `json:"path"`
	Action   Action `json:"action"`
	Upstream string `json:"upstream,omitempty"`
}

func Plan(f Facts, req Request) (Planned, []wtm.Warning, error) {
	branches := make([]BranchPlan, 0, len(req.Branches))
	var invalid []wtm.FieldError
	for index, d := range f.decisions(req) {
		switch decided := d.(type) {
		case refusal:
			if decided.Kind != wtm.KindInvalid {
				return Planned{}, nil, &wtm.Error{Kind: decided.Kind, Code: decided.Code, Message: decided.field(index).Message(branchPath(index)), Params: decided.Params}
			}
			invalid = append(invalid, decided.field(index))
		case BranchPlan:
			branches = append(branches, decided)
		}
	}
	if len(invalid) > 0 {
		return Planned{}, nil, wtm.Invalid(invalid)
	}
	planned := Planned{
		Branches:    branches,
		From:        req.From,
		EnvFrom:     req.EnvFrom,
		Isolation:   f.plannedIsolation(req.Isolation),
		FastForward: f.fastForwarded(req),
		Remember:    req.Remember,
	}
	return planned, append(f.divergedWarnings(req), f.fallbackWarnings(planned)...), nil
}

func (f Facts) plannedIsolation(given project.Isolation) project.Isolation {
	if !f.Run.IsolationApplies() {
		return ""
	}
	return given
}

func (f Facts) fastForwarded(req Request) []string {
	if req.FastForward == nil || !*req.FastForward {
		return nil
	}
	return f.subjectsAt(req, worktree.Behind)
}

func (f Facts) divergedWarnings(req Request) []wtm.Warning {
	var warnings []wtm.Warning
	for _, subject := range f.subjectsAt(req, worktree.Diverged) {
		counts := f.Divergence[subject]
		warnings = append(warnings, wtm.Warning{
			Code:    "create.source_diverged",
			Message: "branch " + subject + " has diverged from origin (" + aheadAndBehind(counts) + ") and can't be fast-forwarded: the worktree starts from your local branch, missing commits that are on origin — you may have to rebase or resolve conflicts later",
			Params:  wtm.Params{"branch": subject, "ahead": strconv.Itoa(counts.Ahead), "behind": strconv.Itoa(counts.Behind)},
		})
	}
	return warnings
}

func (f Facts) fallbackWarnings(planned Planned) []wtm.Warning {
	creates := slices.ContainsFunc(planned.Branches, func(b BranchPlan) bool { return b.Action != ActionUnchanged })
	fallsBack := env.ParentFallsBackToMain(env.ParentFallbackParams{
		Strategy:          planned.EnvFrom,
		HasFiles:          len(f.Config.Env.Files) > 0,
		SourceHasWorktree: f.checkedOutAt(planned.From) != "",
	})
	if !creates || !fallsBack {
		return nil
	}
	return []wtm.Warning{{Code: "create.env_parent_fallback", Message: `The "parent" env strategy needs the source branch checked out to copy its .env; without a worktree it comes from main.`}}
}
