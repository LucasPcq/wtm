package create

import (
	"context"
	"slices"
	"strconv"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/rules/worktree"
)

type Action string

const (
	ActionCreate    Action = "create"
	ActionCheckOut  Action = "check_out"
	ActionResume    Action = "resume"
	ActionUnchanged Action = "unchanged"
)

type Plan struct {
	Branches    []Planned          `json:"branches"`
	From        string             `json:"from,omitempty"`
	EnvFrom     domain.EnvStrategy `json:"env_from"`
	Isolation   domain.Isolation   `json:"isolation"`
	FastForward []string           `json:"fast_forward,omitempty"`
	Remember    []string           `json:"remember,omitempty"`
}

type Planned struct {
	Branch   string `json:"branch"`
	Path     string `json:"path"`
	Action   Action `json:"action"`
	Upstream string `json:"upstream,omitempty"`
}

func plan(_ context.Context, req Request, f Facts) (kernel.Plan[Plan], error) {
	branches := make([]Planned, 0, len(req.Branches))
	for _, branch := range req.Branches {
		planned, err := plannedOf(plannedParams{Request: req, Facts: f, Branch: branch})
		if err != nil {
			return kernel.Plan[Plan]{}, err
		}
		branches = append(branches, planned)
	}
	return kernel.Plan[Plan]{
		Detail: Plan{
			Branches:    branches,
			From:        req.From,
			EnvFrom:     req.EnvFrom,
			Isolation:   isolationOf(req, f),
			FastForward: fastForwarded(req, f),
			Remember:    req.Remember,
		},
		Warnings: planWarnings(planWarningsParams{Request: req, Facts: f, Branches: branches}),
	}, nil
}

type plannedParams struct {
	Request Request
	Facts   Facts
	Branch  string
}

func plannedOf(params plannedParams) (Planned, error) {
	branch := params.Branch
	path := params.Facts.pathOf(branch)
	switch creation := params.Facts.creation(creationParams{Branch: branch, IfNotExists: params.Request.IfNotExists}).(type) {
	case worktree.Unchanged:
		return Planned{Branch: branch, Path: creation.Path, Action: ActionUnchanged}, nil
	case worktree.CheckedOutElsewhere:
		return Planned{}, kernel.Conflict(kernel.Problem{Code: CodeCheckedOutElsewhere, Params: kernel.Params{ParamBranch: branch, kernel.ParamPath: creation.Path}})
	case worktree.PathTaken:
		code, problem, _ := takenProblem(takenProblemParams{Branch: branch, Path: path, Occupant: creation.Occupant})
		return Planned{}, kernel.Conflict(kernel.Problem{Code: code, Params: problem})
	case worktree.Add:
	}
	target := params.Facts.target(branch)
	switch {
	case target.State == domain.BranchTargetExisting:
		return Planned{Branch: branch, Path: path, Action: ActionCheckOut}, nil
	case target.Upstream != "":
		return Planned{Branch: branch, Path: path, Action: ActionResume, Upstream: target.Upstream}, nil
	}
	return Planned{Branch: branch, Path: path, Action: ActionCreate}, nil
}

func isolationOf(req Request, f Facts) domain.Isolation {
	if req.Isolation != "" {
		return req.Isolation
	}
	return f.isolation()
}

func fastForwarded(req Request, f Facts) []string {
	if !req.FastForward {
		return nil
	}
	return behind(req, f)
}

type planWarningsParams struct {
	Request  Request
	Facts    Facts
	Branches []Planned
}

func planWarnings(params planWarningsParams) []kernel.Warning {
	var warnings []kernel.Warning
	for _, subject := range diverged(params.Request, params.Facts) {
		_, counts := params.Facts.divergence(subject)
		warnings = append(warnings, kernel.Warning{Code: CodeSourceDiverged, Params: kernel.Params{
			ParamBranch: subject, ParamAhead: strconv.Itoa(counts.Ahead), ParamBehind: strconv.Itoa(counts.Behind),
		}})
	}
	if parentFallsBack(params) {
		warnings = append(warnings, kernel.Warning{Code: CodeEnvParentFallback})
	}
	return warnings
}

func parentFallsBack(params planWarningsParams) bool {
	creates := slices.ContainsFunc(params.Branches, func(planned Planned) bool { return planned.Action != ActionUnchanged })
	return creates && rules.ParentEnvFallsBackToMain(rules.ParentEnvFallbackParams{
		Strategy:          params.Request.EnvFrom,
		HasCopyFiles:      len(params.Facts.Config.Env.Files) > 0,
		SourceHasWorktree: params.Facts.Identity.CheckedOutAt(params.Request.From) != "",
	})
}
