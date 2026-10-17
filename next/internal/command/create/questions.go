package create

import (
	"slices"
	"strconv"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// ----------------------------------------------------------------------------
// The form

// Questions is the request as a form: each question with its options, its answer so far and where that answer comes from, the fields whose answer is to be remembered, and the request's problems as field errors.
type Questions struct {
	Branches    Branches
	From        From
	EnvFrom     domain.Choice[project.EnvStrategy]
	Isolation   domain.Choice[project.Isolation]
	FastForward domain.Confirm
	IfNotExists domain.Confirm
	Remember    []string
	Errors      []domain.FieldError
}

// Branches is the question of the branches to create: those given, and the branches of the repository to suggest, each badged with whether it exists or is only on origin.
type Branches struct {
	Given   []string
	Options []domain.Option
}

// From is the question of the source branch: its answer, the branches it can start from, and the configured base branch, pinned at the top of the list.
type From struct {
	domain.Answer[string]
	Options []domain.Option
	Pinned  string
}

// Form builds the questions for the request on the facts: a field given keeps its value, an empty one takes its remembered answer (unless the request asks again), its configured value or its default.
func Form(f Facts, req Request) Questions {
	recalled := f.Config.Remembered
	if req.Ask {
		f.Config.Remembered = nil
	}

	// The source is settled first: whether a fast-forward is asked depends on the branch the new ones start from.
	from := f.from(req)
	req.From = from.Value

	questions := Questions{
		Branches:    Branches{Given: req.Branches, Options: f.branchOptions()},
		From:        from,
		EnvFrom:     f.envFrom(req.EnvFrom),
		Isolation:   f.isolation(req.Isolation),
		FastForward: f.fastForward(req),
		IfNotExists: ifNotExists(req.IfNotExists),
		Remember:    req.Remember,
	}
	// Even under --ask, the wizard shows what is remembered, so the user can keep or untick it.
	questions.EnvFrom.Remembered = project.EnvStrategy(recalled[project.RememberEnvStrategy])
	questions.Isolation.Remembered = project.Isolation(recalled[project.RememberIsolation])
	questions.Errors = f.problems(questions)
	return questions
}

func (f Facts) branchOptions() []domain.Option {
	options := make([]domain.Option, 0, len(f.Branches.Local)+len(f.Branches.Remote))
	for _, local := range f.Branches.Local {
		badge := "existing"
		if f.CheckedOutAt(local.Name) != "" {
			badge = "worktree exists"
		}
		options = append(options, domain.Option{Value: local.Name, Badge: badge})
	}

	for _, remote := range f.Branches.Remote {
		if !f.Local(remote.Name) {
			options = append(options, domain.Option{Value: remote.Name, Badge: "on origin"})
		}
	}

	return options
}

// from answers the source with the base branch only when no branch is checked out as it exists, since such a branch's parent must be named.
func (f Facts) from(req Request) From {
	question := From{Options: f.sourceOptions(req.Branches), Pinned: f.Config.Worktrees.BaseBranch}
	switch base := f.Config.Worktrees.BaseBranch; {
	case req.From != "":
		question.Answer = domain.Answer[string]{Value: req.From, Origin: domain.OriginRequest}
	case base != "" && !f.checksOut(req):
		question.Answer = domain.Answer[string]{Value: base, Origin: domain.OriginConfig}
	}

	return question
}

// sourceOptions are the branches a new branch can start from, the branches being created excepted; a branch only on origin is offered as origin/<branch>.
func (f Facts) sourceOptions(creating []string) []domain.Option {
	options := make([]domain.Option, 0, len(f.Branches.Local)+len(f.Branches.Remote))
	for _, local := range f.Branches.Local {
		if !slices.Contains(creating, local.Name) {
			options = append(options, domain.Option{Value: local.Name, Badge: f.badge(local.Name)})
		}
	}

	for _, remote := range f.Branches.Remote {
		if !f.Local(remote.Name) && !slices.Contains(creating, remote.Name) {
			options = append(options, domain.Option{Value: worktree.OriginPrefix + remote.Name})
		}
	}

	return options
}

func (f Facts) badge(name string) string {
	counts := f.AheadBehind(name)
	switch f.Standing(name) {
	case worktree.Behind:
		return strconv.Itoa(counts.Behind) + " behind"
	case worktree.Ahead:
		return strconv.Itoa(counts.Ahead) + " ahead"
	case worktree.Diverged:
		return aheadAndBehind(counts)
	case worktree.NoOrigin, worktree.UpToDate:
	}

	return ""
}

func (f Facts) envFrom(given project.EnvStrategy) domain.Choice[project.EnvStrategy] {
	question := domain.Choice[project.EnvStrategy]{Options: project.EnvStrategies()}
	remembered, found := f.remembered(project.RememberEnvStrategy)
	switch {
	case given != "":
		question.Answer = domain.Answer[project.EnvStrategy]{Value: given, Origin: domain.OriginRequest}
	case found:
		question.Answer = domain.Answer[project.EnvStrategy]{Value: project.EnvStrategy(remembered), Origin: domain.OriginRemembered}
	case f.Config.Env.Strategy != "":
		question.Answer = domain.Answer[project.EnvStrategy]{Value: f.Config.Env.Strategy, Origin: domain.OriginConfig}
	}

	return question
}

func (f Facts) isolation(given project.Isolation) domain.Choice[project.Isolation] {
	question := domain.Choice[project.Isolation]{Options: project.Isolations()}
	if !f.Run.IsolationApplies() {
		question.Skip = "run.toml declares nothing a worktree isolates"
	}

	remembered, found := f.remembered(project.RememberIsolation)
	switch {
	case given != "":
		question.Answer = domain.Answer[project.Isolation]{Value: given, Origin: domain.OriginRequest}
	case found && question.Skip == "":
		question.Answer = domain.Answer[project.Isolation]{Value: project.Isolation(remembered), Origin: domain.OriginRemembered}
	case f.Run.Isolation != "":
		question.Answer = domain.Answer[project.Isolation]{Value: f.Run.Isolation.Effective(), Origin: domain.OriginConfig}
	default:
		question.Answer = domain.Answer[project.Isolation]{Value: project.Isolated, Origin: domain.OriginDefault}
	}

	return question
}

func (f Facts) fastForward(req Request) domain.Confirm {
	question := domain.Confirm{}
	if len(f.subjectsAt(req, worktree.Behind)) == 0 {
		question.Skip = "nothing to fast-forward: no branch it starts from is behind origin"
	}

	remembered, found := f.remembered(project.RememberSourceUpdate)
	switch {
	case req.FastForward != nil:
		question.Answer = domain.Answer[bool]{Value: *req.FastForward, Origin: domain.OriginRequest}
	case question.Skip != "":
	case found:
		question.Answer = domain.Answer[bool]{Value: remembered == project.SourceUpdateFastForward, Origin: domain.OriginRemembered}
	default:
		question.Answer = domain.Answer[bool]{Origin: domain.OriginDefault}
	}

	return question
}

func ifNotExists(given bool) domain.Confirm {
	question := domain.Confirm{Answer: domain.Answer[bool]{Value: given, Origin: domain.OriginDefault}, Skip: "set by its flag only"}
	if given {
		question.Origin = domain.OriginRequest
	}

	return question
}

func (f Facts) remembered(id string) (string, bool) {
	value := f.Config.Remembered[id]
	return value, value != ""
}

// ----------------------------------------------------------------------------
// Resolve and validation

// The field-error codes create spells as every command does, with no create. prefix.
const (
	codeRequired = "required"
	codeOneOf    = "one_of"
)

// remembered pairs each field whose answer can be remembered with its id in config.toml.
var remembered = []struct{ field, id string }{
	{"env_from", project.RememberEnvStrategy},
	{"isolation", project.RememberIsolation},
	{"fast_forward", project.RememberSourceUpdate},
}

func rememberable() []string {
	fields := make([]string, 0, len(remembered))
	for _, r := range remembered {
		fields = append(fields, r.field)
	}

	return fields
}

// Resolve settles the request on the facts, every field answered as Form answers it; a request with problems is refused with all of them, by domain.Invalid.
func Resolve(f Facts, req Request) (Request, error) {
	questions := Form(f, req)
	if len(questions.Errors) > 0 {
		return Request{}, domain.Invalid(questions.Errors)
	}

	return questions.Request(), nil
}

// Request is the request the questions answer; a field whose question is skipped is not remembered.
func (q Questions) Request() Request {
	return Request{
		Branches:    q.Branches.Given,
		From:        q.From.Value,
		EnvFrom:     q.EnvFrom.Value,
		Isolation:   q.Isolation.Value,
		FastForward: new(q.FastForward.Value),
		IfNotExists: q.IfNotExists.Value,
		Remember:    slices.DeleteFunc(slices.Clone(q.Remember), q.skipped),
	}
}

func (q Questions) skipped(field string) bool {
	return field == "isolation" && q.Isolation.Skip != "" || field == "fast_forward" && q.FastForward.Skip != ""
}

func (f Facts) problems(q Questions) []domain.FieldError {
	problems := f.branchProblems(q)
	if from := q.From.Value; from != "" && !f.Candidate(from) {
		problems = append(problems, domain.FieldError{Path: "from", Code: "not_found", Template: "{path}: {value} not found", Params: domain.Params{"value": from}})
	}

	problems = append(problems, oneOf(choice{Path: "env_from", Value: string(q.EnvFrom.Value), Accepted: Names(project.EnvStrategies())})...)
	if q.Isolation.Origin == domain.OriginRequest {
		problems = append(problems, oneOf(choice{Path: "isolation", Value: string(q.Isolation.Value), Accepted: Names(project.Isolations())})...)
	}

	for index, id := range q.Remember {
		problems = append(problems, oneOf(choice{Path: "remember[" + strconv.Itoa(index) + "]", Value: id, Accepted: rememberable()})...)
	}

	return append(problems, f.crossProblems(q)...)
}

// branchProblems are the refusals of the branches given, each as a field error of its branch.
func (f Facts) branchProblems(q Questions) []domain.FieldError {
	branches := q.Branches.Given
	if len(branches) == 0 {
		return []domain.FieldError{{Path: "branches", Code: codeRequired, Template: "{path} is required"}}
	}

	var problems []domain.FieldError
	for index, d := range f.decisions(Request{Branches: branches, IfNotExists: q.IfNotExists.Value}) {
		if refused, isRefused := d.(refusal); isRefused {
			problems = append(problems, refused.field(index))
		}
	}

	return problems
}

// crossProblems are the problems of the source against the branches: a branch checked out as it exists needs its parent named, and no branch is its own parent.
func (f Facts) crossProblems(q Questions) []domain.FieldError {
	var problems []domain.FieldError
	branches, from := q.Branches.Given, q.From.Value
	if from == "" && f.checksOut(Request{Branches: branches, IfNotExists: q.IfNotExists.Value}) {
		problems = append(problems, domain.FieldError{
			Path:     "from",
			Code:     codeRequired,
			Template: "{path} is required: a branch that already exists locally needs its parent named (it can't be inferred, and `wtm sync` needs it)",
			Params:   domain.Params{"because": "create.parent_required"},
		})
	}

	if from != "" && slices.Contains(branches, from) {
		problems = append(problems, domain.FieldError{Path: "from", Code: "self_parent", Template: "{value} cannot be its own parent: pass another value to {path}", Params: domain.Params{"value": from}, With: []string{"branches"}})
	}

	return problems
}

type choice struct {
	Path     string
	Value    string
	Accepted []string
}

// oneOf is the problem of a value that is not one of those accepted; an empty value is none.
func oneOf(c choice) []domain.FieldError {
	if c.Value == "" || slices.Contains(c.Accepted, c.Value) {
		return nil
	}

	return []domain.FieldError{{
		Path:     c.Path,
		Code:     codeOneOf,
		Template: "{path} must be one of {accepted}, not {value}",
		Params:   domain.Params{"value": c.Value},
		Accepted: c.Accepted,
	}}
}

// Names spells each of values as a string, in order: the accepted values of a field, for its validation and its completion.
func Names[T ~string](values []T) []string {
	found := make([]string, 0, len(values))
	for _, value := range values {
		found = append(found, string(value))
	}

	return found
}

func branchPath(index int) string {
	return "branches[" + strconv.Itoa(index) + "]"
}
