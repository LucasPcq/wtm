package create

import (
	"slices"
	"strconv"

	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/envcopy"
	"github.com/LucasPcq/wtm/next/internal/wtm/project"
	"github.com/LucasPcq/wtm/next/internal/wtm/run"
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
	Isolation   run.Isolation       `json:"isolation,omitempty"`
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
	for _, d := range f.decisions(req) {
		switch decided := d.(type) {
		case refusal:
			return Planned{}, nil, &wtm.Error{Kind: wtm.KindConflict, Code: decided.Code, Message: decided.Message, Params: decided.Params}
		case BranchPlan:
			branches = append(branches, decided)
		}
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

func (f Facts) isolationOf(given run.Isolation) run.Isolation {
	if given != "" {
		return given
	}
	return f.Run.Isolation.Effective()
}

func (f Facts) plannedIsolation(given run.Isolation) run.Isolation {
	if !f.Run.IsolationApplies() {
		return ""
	}
	return f.isolationOf(given)
}

func (f Facts) fastForwarded(req Request) []string {
	if req.FastForward == nil || !*req.FastForward {
		return nil
	}
	return f.behind(req)
}

func (f Facts) divergedWarnings(req Request) []wtm.Warning {
	var warnings []wtm.Warning
	for _, subject := range f.diverged(req) {
		counts := f.Divergence[subject]
		ahead, behind := strconv.Itoa(counts.Ahead), strconv.Itoa(counts.Behind)
		warnings = append(warnings, wtm.Warning{
			Code:    "create.source_diverged",
			Message: "branch " + subject + " has diverged from origin (" + ahead + " ahead, " + behind + " behind) and can't be fast-forwarded: the worktree starts from your local branch, missing commits that are on origin — you may have to rebase or resolve conflicts later",
			Params:  wtm.Params{"branch": subject, "ahead": ahead, "behind": behind},
		})
	}
	return warnings
}

func (f Facts) fallbackWarnings(planned Planned) []wtm.Warning {
	creates := slices.ContainsFunc(planned.Branches, func(b BranchPlan) bool { return b.Action != ActionUnchanged })
	fallsBack := envcopy.ParentFallsBackToMain(envcopy.ParentFallbackParams{
		Strategy:          planned.EnvFrom,
		HasFiles:          len(f.Config.Env.Files) > 0,
		SourceHasWorktree: f.checkedOutAt(planned.From) != "",
	})
	if !creates || !fallsBack {
		return nil
	}
	return []wtm.Warning{{Code: "create.env_parent_fallback", Message: `The "parent" env strategy needs the source branch checked out to copy its .env; without a worktree it comes from main.`}}
}
