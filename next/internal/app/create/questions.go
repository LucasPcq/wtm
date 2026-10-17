package create

import (
	"slices"
	"strconv"

	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/branch"
	"github.com/LucasPcq/wtm/next/internal/wtm/project"
	"github.com/LucasPcq/wtm/next/internal/wtm/run"
)

type Origin string

const (
	OriginRequest    Origin = "request"
	OriginRemembered Origin = "remembered"
	OriginConfig     Origin = "config"
	OriginDefault    Origin = "default"
)

type Answer[T any] struct {
	Value  T
	Origin Origin
}

type Option struct {
	Value string
	Badge string
}

type Questions struct {
	Branches    Branches
	From        From
	EnvFrom     Choice[project.EnvStrategy]
	Isolation   Choice[run.Isolation]
	FastForward Confirm
	IfNotExists bool
	Remember    []string
	Errors      []wtm.FieldError
}

type Branches struct {
	Given   []string
	Options []Option
}

type From struct {
	Answer[string]
	Options []Option
	Pinned  string
}

type Choice[T ~string] struct {
	Answer[T]
	Options []T
	Skip    string
}

type Confirm struct {
	Answer[bool]
	Skip string
}

func Form(f Facts, req Request) Questions {
	if req.Ask {
		f.Config.Remembered = nil
	}
	from := f.from(req)
	req.From = from.Value
	questions := Questions{
		Branches:    Branches{Given: req.Branches, Options: f.branchOptions()},
		From:        from,
		EnvFrom:     f.envFrom(req.EnvFrom),
		Isolation:   f.isolation(req.Isolation),
		FastForward: f.fastForward(req),
		IfNotExists: req.IfNotExists,
		Remember:    req.Remember,
	}
	questions.Errors = f.problems(questions)
	return questions
}

func (f Facts) branchOptions() []Option {
	options := make([]Option, 0, len(f.Branches.Local)+len(f.Branches.Remote))
	for _, local := range f.Branches.Local {
		badge := "existing"
		if f.checkedOutAt(local.Name) != "" {
			badge = "worktree exists"
		}
		options = append(options, Option{Value: local.Name, Badge: badge})
	}
	for _, remote := range f.Branches.Remote {
		if !f.local(remote.Name) {
			options = append(options, Option{Value: remote.Name, Badge: "on origin"})
		}
	}
	return options
}

func (f Facts) from(req Request) From {
	question := From{Options: f.sourceOptions(req.Branches), Pinned: f.Config.Worktrees.BaseBranch}
	switch base := f.Config.Worktrees.BaseBranch; {
	case req.From != "":
		question.Answer = Answer[string]{Value: req.From, Origin: OriginRequest}
	case base != "" && !f.checksOut(req):
		question.Answer = Answer[string]{Value: base, Origin: OriginConfig}
	}
	return question
}

func (f Facts) sourceOptions(creating []string) []Option {
	options := make([]Option, 0, len(f.Branches.Local)+len(f.Branches.Remote))
	for _, local := range f.Branches.Local {
		if !slices.Contains(creating, local.Name) {
			options = append(options, Option{Value: local.Name, Badge: f.badge(local.Name)})
		}
	}
	for _, remote := range f.Branches.Remote {
		if !f.local(remote.Name) && !slices.Contains(creating, remote.Name) {
			options = append(options, Option{Value: remotePrefix + remote.Name})
		}
	}
	return options
}

func (f Facts) badge(name string) string {
	counts := f.Divergence[name]
	switch f.standing(name) {
	case branch.Behind:
		return strconv.Itoa(counts.Behind) + " behind"
	case branch.Ahead:
		return strconv.Itoa(counts.Ahead) + " ahead"
	case branch.Diverged:
		return aheadAndBehind(counts)
	case branch.NoOrigin, branch.UpToDate:
	}
	return ""
}

func (f Facts) envFrom(given project.EnvStrategy) Choice[project.EnvStrategy] {
	question := Choice[project.EnvStrategy]{Options: project.EnvStrategies()}
	remembered, found := f.remembered(project.RememberEnvStrategy)
	switch {
	case given != "":
		question.Answer = Answer[project.EnvStrategy]{Value: given, Origin: OriginRequest}
	case found:
		question.Answer = Answer[project.EnvStrategy]{Value: project.EnvStrategy(remembered), Origin: OriginRemembered}
	case f.Config.Env.Strategy != "":
		question.Answer = Answer[project.EnvStrategy]{Value: f.Config.Env.Strategy, Origin: OriginConfig}
	}
	return question
}

func (f Facts) isolation(given run.Isolation) Choice[run.Isolation] {
	question := Choice[run.Isolation]{Options: run.Isolations()}
	if !f.Run.IsolationApplies() {
		question.Skip = "run.toml declares nothing a worktree isolates"
	}
	remembered, found := f.remembered(project.RememberIsolation)
	switch {
	case given != "":
		question.Answer = Answer[run.Isolation]{Value: given, Origin: OriginRequest}
	case found && question.Skip == "":
		question.Answer = Answer[run.Isolation]{Value: run.Isolation(remembered), Origin: OriginRemembered}
	case f.Run.Isolation != "":
		question.Answer = Answer[run.Isolation]{Value: f.Run.Isolation.Effective(), Origin: OriginConfig}
	default:
		question.Answer = Answer[run.Isolation]{Value: run.Isolated, Origin: OriginDefault}
	}
	return question
}

func (f Facts) fastForward(req Request) Confirm {
	question := Confirm{}
	if len(f.subjectsAt(req, branch.Behind)) == 0 {
		question.Skip = "nothing to fast-forward: no branch it starts from is behind origin"
	}
	remembered, found := f.remembered(project.RememberSourceUpdate)
	switch {
	case req.FastForward != nil:
		question.Answer = Answer[bool]{Value: *req.FastForward, Origin: OriginRequest}
	case question.Skip != "":
	case found:
		question.Answer = Answer[bool]{Value: remembered == project.SourceUpdateFastForward, Origin: OriginRemembered}
	default:
		question.Answer = Answer[bool]{Origin: OriginDefault}
	}
	return question
}

func (f Facts) remembered(id string) (string, bool) {
	value := f.Config.Remembered[id]
	return value, value != ""
}
