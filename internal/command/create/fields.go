package create

import (
	"context"
	"path/filepath"
	"slices"
	"strconv"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra/git"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/rules/worktree"
)

const (
	labelSource      = "Source branch"
	labelEnv         = "Env strategy"
	labelIfNotExists = "If not exists"
	describeBranches = "The branches to create a worktree for; one that already exists is checked out as it is"
	describeSource   = "The branch a new branch starts from, or the parent recorded for wtm sync of one that already exists"
	describeEnv      = "How the .env files of the new worktrees are provisioned"
	describeFF       = "Fast-forward to origin first the branch each worktree starts from, when it is behind"
	describeIfNot    = "Succeed without a change when the worktree already exists (idempotent)"
	describeRemember = "The answers to always use in this repository"
	updateFF         = "ff"
	updateKeep       = "keep"
)

var (
	envStrategies = []string{string(domain.EnvStrategyExample), string(domain.EnvStrategyMain), string(domain.EnvStrategyParent)}
	isolations    = []string{string(domain.IsolationIsolated), string(domain.IsolationVerbatim)}
)

func fields() []kernel.FieldDef[Request, Facts] {
	answers := []kernel.FieldDef[Request, Facts]{branchesField(), fromField(), envField(), isolationField(), fastForwardField(), ifNotExistsField()}
	return append(answers, kernel.Remember(kernel.RememberParams[Request, Facts]{Fields: answers, Label: domain.RememberToggleLabel, Description: describeRemember}))
}

func branchesField() kernel.FieldDef[Request, Facts] {
	return kernel.FieldDef[Request, Facts]{
		Spec: kernel.FieldSpec{
			Path: PathBranches, Type: kernel.FieldTextList, Label: domain.CreateBranchesLabel, Description: describeBranches,
			Required: true, DependsOn: []string{PathIfNotExists}, Choices: kernel.ChoicesDynamic,
		},
		Validate: entryProblems,
		Choices:  branchChoices,
	}
}

func entryProblems(req Request, f Facts) []kernel.FieldError {
	var problems []kernel.FieldError
	for index, entry := range req.Branches {
		if slices.Contains(req.Branches[:index], entry) {
			continue
		}
		code, params, found := entryProblem(entryParams{Entry: entry, Earlier: req.Branches[:index], IfNotExists: req.IfNotExists, Facts: f})
		if found {
			problems = append(problems, kernel.FieldError{Path: kernel.IndexPath(PathBranches, index), Code: code, Params: params})
		}
	}
	return problems
}

type entryParams struct {
	Entry       string
	Earlier     []string
	IfNotExists bool
	Facts       Facts
}

func entryProblem(params entryParams) (kernel.Code, kernel.Params, bool) {
	if err := rules.BranchNameProblem(params.Entry); err != nil {
		return CodeBranchNameInvalid, kernel.Params{ParamProblem: err.Error()}, true
	}
	if clash, found := listClash(params); found {
		return CodeNameClash, clashParams(params.Entry, clash), true
	}
	switch creation := params.Facts.creation(creationParams{Branch: params.Entry, IfNotExists: params.IfNotExists}).(type) {
	case worktree.CheckedOutElsewhere:
		return CodeCheckedOutElsewhere, kernel.Params{ParamBranch: params.Entry, kernel.ParamPath: creation.Path}, true
	case worktree.PathTaken:
		return takenProblem(takenProblemParams{Branch: params.Entry, Path: params.Facts.pathOf(params.Entry), Occupant: creation.Occupant})
	case worktree.Unchanged:
		return "", nil, false
	case worktree.Add:
	}
	if !params.Facts.derivedNames() {
		return "", nil, false
	}
	if clash, found := params.Facts.Identity.NameClash(params.Entry); found {
		return CodeNameClash, clashParams(params.Entry, clash), true
	}
	return "", nil, false
}

func listClash(params entryParams) (domain.WorktreeNameClash, bool) {
	if !params.Facts.derivedNames() {
		return domain.WorktreeNameClash{}, false
	}
	return rules.WorktreeNameClash(rules.WorktreeNameClashParams{Branch: params.Entry, Live: params.Earlier})
}

func clashParams(branch string, clash domain.WorktreeNameClash) kernel.Params {
	return kernel.Params{ParamBranch: branch, ParamOther: clash.Branch, ParamName: clash.Name}
}

type takenProblemParams struct {
	Branch   string
	Path     string
	Occupant string
}

func takenProblem(params takenProblemParams) (kernel.Code, kernel.Params, bool) {
	if params.Occupant == "" || params.Occupant == params.Branch {
		return CodePathExists, kernel.Params{kernel.ParamPath: params.Path}, true
	}
	return CodeNameClash, kernel.Params{ParamBranch: params.Branch, ParamOther: params.Occupant, ParamName: filepath.Base(params.Path)}, true
}

func branchChoices(_ context.Context, _ Request, f Facts) (kernel.Choices, error) {
	options := make([]kernel.Option, 0, len(f.Branches.Local)+len(f.Branches.Remote))
	for _, branch := range f.Branches.Local {
		badge := CodeBadgeExisting
		if f.Identity.CheckedOutAt(branch.Name) != "" {
			badge = CodeBadgeCheckedOut
		}
		options = append(options, kernel.Option{Value: branch.Name, Label: branch.Name, Badges: []kernel.Badge{{Code: badge}}})
	}
	for _, branch := range f.Branches.Remote {
		if !f.local(branch.Name) {
			options = append(options, kernel.Option{Value: branch.Name, Label: branch.Name, Badges: []kernel.Badge{{Code: CodeBadgeOnOrigin}}})
		}
	}
	return kernel.Choices{Options: options}, nil
}

func fromField() kernel.FieldDef[Request, Facts] {
	return kernel.FieldDef[Request, Facts]{
		Spec: kernel.FieldSpec{
			Path: PathFrom, Type: kernel.FieldSelect, Label: labelSource, Description: describeSource,
			DependsOn: []string{PathBranches}, Choices: kernel.ChoicesSearch,
		},
		Default:  defaultSource,
		Validate: knownSource,
		Choices:  sourceChoices,
	}
}

func defaultSource(req Request, f Facts) (kernel.Fallback, bool) {
	base := f.Config.Worktrees.BaseBranch
	if base == "" || anyExisting(req, f) {
		return kernel.Fallback{}, false
	}
	return kernel.Fallback{Value: kernel.Text(base), Origin: kernel.OriginConfig}, true
}

func anyExisting(req Request, f Facts) bool {
	return slices.ContainsFunc(req.Branches, func(branch string) bool {
		return rules.ParentMustBeExplicit(f.target(branch).State)
	})
}

func knownSource(req Request, f Facts) []kernel.FieldError {
	if req.From == "" || f.candidate(req.From) {
		return nil
	}
	return []kernel.FieldError{{Path: PathFrom, Code: kernel.CodeNotFound, Params: kernel.Params{kernel.ParamValue: req.From}}}
}

func sourceChoices(_ context.Context, req Request, f Facts) (kernel.Choices, error) {
	options := make([]kernel.Option, 0, len(f.Branches.Local)+len(f.Branches.Remote))
	for _, branch := range f.Branches.Local {
		if !slices.Contains(req.Branches, branch.Name) {
			options = append(options, kernel.Option{Value: branch.Name, Label: branch.Name, Badges: f.divergenceBadges(branch.Name)})
		}
	}
	for _, branch := range f.Branches.Remote {
		if !f.local(branch.Name) && !slices.Contains(req.Branches, branch.Name) {
			name := domain.RemoteBranchPrefix + branch.Name
			options = append(options, kernel.Option{Value: name, Label: name})
		}
	}
	return kernel.Choices{Options: options, Pinned: f.Config.Worktrees.BaseBranch}, nil
}

func (f Facts) divergenceBadges(branch string) []kernel.Badge {
	state, counts := f.divergence(branch)
	params := kernel.Params{ParamAhead: strconv.Itoa(counts.Ahead), ParamBehind: strconv.Itoa(counts.Behind)}
	switch state {
	case domain.DivergenceBehind:
		return []kernel.Badge{{Code: CodeBadgeBehind, Params: params}}
	case domain.DivergenceAhead:
		return []kernel.Badge{{Code: CodeBadgeAhead, Params: params}}
	case domain.DivergenceDiverged:
		return []kernel.Badge{{Code: CodeBadgeDiverged, Params: params}}
	case domain.DivergenceUpToDate, domain.DivergenceUnknown:
	}
	return nil
}

func envField() kernel.FieldDef[Request, Facts] {
	return kernel.FieldDef[Request, Facts]{
		Spec: kernel.FieldSpec{
			Path: PathEnvFrom, Type: kernel.FieldSelect, Label: labelEnv, Description: describeEnv,
			Rememberable: true, Constraints: kernel.Constraints{Enum: envStrategies},
		},
		Default: func(_ Request, f Facts) (kernel.Fallback, bool) {
			if value, found := f.remembered(domain.RememberEnvStrategy); found {
				return kernel.Fallback{Value: kernel.Text(value), Origin: kernel.OriginRemembered}, true
			}
			strategy := f.Config.Env.Strategy
			return kernel.Fallback{Value: kernel.Text(strategy), Origin: kernel.OriginConfig}, strategy != ""
		},
	}
}

func isolationField() kernel.FieldDef[Request, Facts] {
	return kernel.FieldDef[Request, Facts]{
		Spec: kernel.FieldSpec{
			Path: PathIsolation, Type: kernel.FieldSelect, Label: domain.IsolationStepName, Description: domain.IsolationStepDescription,
			Rememberable: true, Constraints: kernel.Constraints{Enum: isolations},
		},
		Skip: func(_ Request, f Facts) (bool, kernel.Code) {
			return !rules.IsolationApplies(f.Run), CodeNothingToIsolate
		},
		Default: func(_ Request, f Facts) (kernel.Fallback, bool) {
			if value, found := f.remembered(domain.RememberIsolation); found {
				return kernel.Fallback{Value: kernel.Text(value), Origin: kernel.OriginRemembered}, true
			}
			if f.Run.Isolation != "" {
				return kernel.Fallback{Value: kernel.Text(f.isolation()), Origin: kernel.OriginConfig}, true
			}
			return kernel.Fallback{Value: kernel.Text(domain.IsolationIsolated), Origin: kernel.OriginDefault}, true
		},
	}
}

func fastForwardField() kernel.FieldDef[Request, Facts] {
	return kernel.FieldDef[Request, Facts]{
		Spec: kernel.FieldSpec{
			Path: PathFastForward, Type: kernel.FieldBool, Label: domain.SourceUpdateLabel, Description: describeFF,
			Rememberable: true, DependsOn: []string{PathBranches, PathFrom},
		},
		Skip: func(req Request, f Facts) (bool, kernel.Code) {
			return len(behind(req, f)) == 0, CodeNothingBehind
		},
		Default: func(_ Request, f Facts) (kernel.Fallback, bool) {
			if value, found := f.remembered(domain.RememberSourceUpdate); found {
				return kernel.Fallback{Value: kernel.Bool(value == updateFF), Origin: kernel.OriginRemembered}, true
			}
			return kernel.Fallback{Value: kernel.Bool(false), Origin: kernel.OriginDefault}, true
		},
	}
}

func ifNotExistsField() kernel.FieldDef[Request, Facts] {
	return kernel.FieldDef[Request, Facts]{
		Spec: kernel.FieldSpec{Path: PathIfNotExists, Type: kernel.FieldBool, Label: labelIfNotExists, Description: describeIfNot},
		Skip: func(Request, Facts) (bool, kernel.Code) { return true, CodeSetByFlag },
	}
}

func checks() kernel.Rules[Request, Facts] {
	return kernel.Rules[Request, Facts]{}.
		Distinct(PathBranches).
		RequiredWhen(PathFrom, kernel.Condition[Request, Facts]{Code: CodeParentRequired, Holds: anyExisting}).
		NotSelfParent(kernel.SelfParentRule{Parent: PathFrom, Children: PathBranches})
}

func sourceRef(f Facts, from string) git.Ref {
	if remote, isRemote := cutRemote(from); isRemote && !f.local(from) {
		return git.RemoteBranch{Name: remote}
	}
	return git.LocalBranch{Name: from}
}
