package create

import (
	"slices"
	"strconv"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// ----------------------------------------------------------------------------
// The request

// A Request is what the caller asks of create, as given: a field left empty takes its remembered answer, its configured value or its default when the request is resolved.
type Request struct {
	Branches    []string            `json:"branches"`
	From        string              `json:"from"`
	EnvFrom     project.EnvStrategy `json:"env_from"`
	Isolation   project.Isolation   `json:"isolation"`
	FastForward *bool               `json:"fast_forward"` // nil when not given, so a remembered answer can apply
	IfNotExists bool                `json:"if_not_exists"`
	Remember    []string            `json:"remember"` // the fields whose answer is remembered for the next runs
	Ask         bool                `json:"ask"`      // ignore the remembered answers for this run
}

// Answer is the request with the field at path answered by values, spelled as the field's Answer spells them; the path "remember" answers the fields whose answer is remembered. A value the field cannot take leaves the request as it was.
func (r Request) Answer(path string, values []string) (Request, error) {
	answer := domain.AnswerParams{Path: path, Values: values}
	next := r
	var err error
	switch path {
	case "branches":
		next.Branches = values
	case domain.PathRemember:
		next.Remember = values
	case "from":
		next.From, err = domain.One[string](answer)
	case "env_from":
		next.EnvFrom, err = domain.One[project.EnvStrategy](answer)
	case "isolation":
		next.Isolation, err = domain.One[project.Isolation](answer)
	case "fast_forward":
		var yes bool
		yes, err = domain.Yes(answer)
		next.FastForward = &yes
	case "if_not_exists":
		next.IfNotExists, err = domain.Yes(answer)
	default:
		err = domain.Unknown(path)
	}

	if err != nil {
		return r, err
	}

	return next, nil
}

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

// NewQuestions builds the questions for the request on the facts: a field given keeps its value, an empty one takes its remembered answer (unless the request asks again), its configured value or its default.
func NewQuestions(f Facts, req Request) Questions {
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
	questions.EnvFrom.Remembered = project.RememberedEnvStrategy(recalled[project.RememberEnvStrategy], f.Config.Env.Strategy)
	questions.Isolation.Remembered = project.Isolation(recalled[project.RememberIsolation])
	questions.Errors = f.problems(questions)
	return questions
}

// Form is the questions as the schema a surface renders: the fields in the order the wizard asks them, each with its flag, label, help and answer so far. A zero Questions gives the schema the flags and --help are spelled from.
func (q Questions) Form() domain.Form {
	return domain.Form{Problems: q.Errors, Fields: []domain.Field{
		domain.TextListField{
			FieldSpec: domain.FieldSpec{Path: "branches", Flag: "<branches>", Label: "Branches", Help: "The branches to create a worktree for; one that already exists is checked out as it is"},
			Entries:   q.Branches.Given,
			Options:   q.Branches.Options,
		},
		domain.SelectField{
			FieldSpec: domain.FieldSpec{Path: "from", Flag: "--from", Label: "Source branch", Help: "The branch a new branch starts from, or the parent recorded for wtm sync of one that already exists", Origin: q.From.Origin},
			Value:     q.From.Value,
			Options:   q.From.Options,
			Pinned:    q.From.Pinned,
		},
		domain.SelectField{
			FieldSpec:    domain.FieldSpec{Path: "env_from", Flag: "--env-from", Label: "Env strategy", Help: "How the .env files of the new worktrees are provisioned: example (from its template), main (copied from the main checkout) or parent (from the source's worktree); defaults to config.toml's strategy", Origin: q.EnvFrom.Origin},
			Value:        string(q.EnvFrom.Value),
			Options:      domain.Options(project.EnvStrategies()),
			Remembered:   string(q.EnvFrom.Remembered),
			Rememberable: true,
			Remember:     q.remembers("env_from"),
		},
		domain.SelectField{
			FieldSpec:    domain.FieldSpec{Path: "isolation", Flag: "--isolation", Label: "Isolation", Help: domain.IsolationHelp, Origin: q.Isolation.Origin, Skip: q.Isolation.Skip},
			Value:        string(q.Isolation.Value),
			Options:      domain.Options(project.Isolations()),
			Remembered:   string(q.Isolation.Remembered),
			Rememberable: true,
			Remember:     q.remembers("isolation"),
		},
		domain.ConfirmField{
			FieldSpec:    domain.FieldSpec{Path: "fast_forward", Flag: "--ff", Label: "Source update", Help: "Fast-forward to origin first the branch each worktree starts from, when it is behind; one that has diverged or holds uncommitted changes starts as it is", Origin: q.FastForward.Origin, Skip: q.FastForward.Skip},
			Value:        q.FastForward.Value,
			Rememberable: true,
			Remember:     q.remembers("fast_forward"),
		},
		domain.ConfirmField{
			FieldSpec: domain.FieldSpec{Path: "if_not_exists", Flag: "--if-not-exists", Label: "If not exists", Help: "Succeed without a change when the worktree already exists (idempotent)", Origin: q.IfNotExists.Origin, Skip: q.IfNotExists.Skip},
			Value:     q.IfNotExists.Value,
		},
	}}
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
			options = append(options, domain.Option{Value: local.Name, Badge: f.Badge(local.Name)})
		}
	}

	for _, remote := range f.Branches.Remote {
		if !f.Local(remote.Name) && !slices.Contains(creating, remote.Name) {
			options = append(options, domain.Option{Value: worktree.OriginPrefix + remote.Name})
		}
	}

	return options
}

func (f Facts) envFrom(given project.EnvStrategy) domain.Choice[project.EnvStrategy] {
	question := domain.Choice[project.EnvStrategy]{Options: project.EnvStrategies()}
	remembered := project.RememberedEnvStrategy(f.Config.Remembered[project.RememberEnvStrategy], f.Config.Env.Strategy)
	switch {
	case given != "":
		question.Answer = domain.Answer[project.EnvStrategy]{Value: given, Origin: domain.OriginRequest}
	case remembered != "":
		question.Answer = domain.Answer[project.EnvStrategy]{Value: remembered, Origin: domain.OriginRemembered}
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

	question.Answer = domain.IsolationAnswer(project.Isolation(f.Config.Remembered[project.RememberIsolation]), f.Run)
	if given != "" {
		question.Answer = domain.Answer[project.Isolation]{Value: given, Origin: domain.OriginRequest}
	}

	return question
}

func (f Facts) fastForward(req Request) domain.Confirm {
	params := domain.FastForwardParams{Given: req.FastForward, Remembered: f.Config.Remembered[project.RememberSourceUpdate]}
	if len(f.subjectsAt(req, worktree.Behind)) == 0 {
		params.Skip = "nothing to fast-forward: no branch it starts from is behind origin"
	}

	return domain.FastForwardAnswer(params)
}

func ifNotExists(given bool) domain.Confirm {
	question := domain.Confirm{Answer: domain.Answer[bool]{Value: given, Origin: domain.OriginDefault}, Skip: "set by its flag only"}
	if given {
		question.Origin = domain.OriginRequest
	}

	return question
}

// ----------------------------------------------------------------------------
// Resolve and validation

// codeRequired is the field-error code create spells as every command does, with no create. prefix.
const codeRequired = "required"

// rememberable are the fields whose answer can be remembered.
func rememberable() []string {
	return Questions{}.Form().Rememberable()
}

// Resolve settles the request on the facts, every field answered as NewQuestions answers it; a request with problems is refused with all of them, by domain.Invalid.
func Resolve(f Facts, req Request) (Request, error) {
	questions := NewQuestions(f, req)
	if len(questions.Errors) > 0 {
		return Request{}, domain.Invalid(questions.Errors)
	}

	return questions.Request(), nil
}

// Request is the request the questions answer.
func (q Questions) Request() Request {
	return Request{
		Branches:    q.Branches.Given,
		From:        q.From.Value,
		EnvFrom:     q.EnvFrom.Value,
		Isolation:   q.Isolation.Value,
		FastForward: new(q.FastForward.Value),
		IfNotExists: q.IfNotExists.Value,
		Remember:    q.remembered(),
	}
}

// remembered are the fields whose answer is to be remembered, in the order of the form and each once; a field whose question is skipped is not.
func (q Questions) remembered() []string {
	var remembered []string
	for _, field := range rememberable() {
		if q.remembers(field) {
			remembered = append(remembered, field)
		}
	}

	return remembered
}

func (q Questions) remembers(field string) bool {
	return slices.Contains(q.Remember, field) && !q.skipped(field)
}

func (q Questions) skipped(field string) bool {
	return field == "isolation" && q.Isolation.Skip != "" || field == "fast_forward" && q.FastForward.Skip != ""
}

func (f Facts) problems(q Questions) []domain.FieldError {
	problems := f.branchProblems(q)
	if from := q.From.Value; from != "" && !f.Candidate(from) {
		problems = append(problems, domain.FieldError{Path: "from", Code: "not_found", Template: "{path}: {value} not found", Params: domain.Params{"value": from}})
	}

	problems = append(problems, domain.OneOf(domain.OneOfParams{Path: "env_from", Value: string(q.EnvFrom.Value), Accepted: domain.Names(project.EnvStrategies())})...)
	if q.Isolation.Origin == domain.OriginRequest {
		problems = append(problems, domain.OneOf(domain.OneOfParams{Path: "isolation", Value: string(q.Isolation.Value), Accepted: domain.Names(project.Isolations())})...)
	}

	for index, id := range q.Remember {
		problems = append(problems, domain.OneOf(domain.OneOfParams{Path: "remember[" + strconv.Itoa(index) + "]", Value: id, Accepted: rememberable()})...)
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

func branchPath(index int) string {
	return "branches[" + strconv.Itoa(index) + "]"
}
