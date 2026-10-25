package create

import (
	"slices"
	"strconv"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
)

// ----------------------------------------------------------------------------
// The request

// A Request is what the caller asks of create, as given: a field left empty takes its remembered answer, its configured value or its default when the request is resolved.
type Request struct {
	Branches []string `json:"branches"`
	From     string   `json:"from"`
	provision.Given
	IfNotExists bool `json:"if_not_exists"`
}

// The paths of create's own fields; the questions of a new worktree name theirs in provision.
const (
	pathBranches    = "branches"
	pathFrom        = "from"
	pathIfNotExists = "if_not_exists"
)

// Answer is the request with the field at path answered by values, spelled as the field's Answer spells them; the path "remember" answers the fields whose answer is remembered. A value the field cannot take leaves the request as it was.
func (r Request) Answer(path string, values []string) (Request, error) {
	next := r
	var err error
	switch path {
	case pathBranches:
		next.Branches = values
	case pathFrom:
		next.From, err = domain.One[string](path, values)
	case pathIfNotExists:
		next.IfNotExists, err = domain.Yes(path, values)
	default:
		next.Given, err = r.Given.Answer(path, values)
	}

	if err != nil {
		return r, err
	}

	return next, nil
}

// Values is the request's answer to the field at path, spelled as Answer takes it; nil when the request leaves the field to its default.
func (r Request) Values(path string) []string {
	switch path {
	case pathBranches:
		return r.Branches
	case pathFrom:
		return domain.Spelled(r.From)
	case pathIfNotExists:
		return domain.SaidYes(r.IfNotExists)
	}

	return r.Given.Values(path)
}

// ----------------------------------------------------------------------------
// The form

// Questions is the request as a form: each question with its options, its answer so far and where that answer comes from, the questions of a new worktree with the fields whose answer is to be remembered, and the request's problems as field errors.
type Questions struct {
	Branches Branches
	From     From
	provision.Questions
	IfNotExists domain.Confirm
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
	// The source is settled first: whether a fast-forward is asked depends on the branch the new ones start from.
	from := f.from(req)
	req.From = from.Value

	questions := Questions{
		Branches:    Branches{Given: req.Branches, Options: f.branchOptions()},
		From:        from,
		Questions:   f.newWorktree(req),
		IfNotExists: ifNotExists(req.IfNotExists),
	}
	questions.Errors = f.problems(questions)
	return questions
}

// Form is the questions as the schema a surface renders: the fields in the order the wizard asks them, each with its flag, label, help and answer so far. A zero Questions gives the schema the flags and --help are spelled from.
func (q Questions) Form() domain.Form {
	fields := []domain.Field{
		domain.TextListField{
			FieldSpec: domain.FieldSpec{Path: pathBranches, Flag: "<branches>", Label: "Branches", Help: "One per line; a branch that exists is checked out as it is"},
			Entries:   q.Branches.Given,
			Options:   q.Branches.Options,
		},
		domain.SelectField{
			FieldSpec:   domain.FieldSpec{Path: pathFrom, Flag: "--from", Label: "Source branch", Help: "Where a new branch starts, and the parent wtm sync rebases it onto", Origin: q.From.Origin},
			Value:       q.From.Value,
			Options:     q.From.Options,
			Pinned:      q.From.Pinned,
			Refreshable: true,
		},
	}
	fields = append(fields, q.Questions.Fields(envHelp)...)
	fields = append(fields, domain.ConfirmField{
		FieldSpec: domain.FieldSpec{Path: pathIfNotExists, Flag: "--if-not-exists", Label: "If not exists", Help: "Succeed with no change when the worktree exists", Origin: q.IfNotExists.Origin, Skip: q.IfNotExists.Skip},
		Value:     q.IfNotExists.Value,
	})

	return domain.Form{Problems: q.Errors, Fields: fields}
}

const envHelp = "Where its .env files come from: config, example, main or parent; config.toml's strategy by default"

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
	question := From{Options: domain.SourceOptions(f.Repository, req.Branches...), Pinned: f.Config.Worktrees.BaseBranch}
	switch base := f.Config.Worktrees.BaseBranch; {
	case req.From != "":
		question.Answer = domain.Answer[string]{Value: req.From, Origin: domain.OriginRequest}
	case base != "" && !f.checksOut(req):
		question.Answer = domain.Answer[string]{Value: base, Origin: domain.OriginConfig}
	}

	return question
}

// newWorktree are the questions of the new worktrees, on the facts; the fast-forward is asked while a branch they start from is behind origin.
func (f Facts) newWorktree(req Request) provision.Questions {
	return provision.NewQuestions(provision.QuestionsParams{Given: req.Given, Repository: f.Repository, Behind: f.subjectsAt(req, worktree.Behind), Many: len(req.Branches) > 1})
}

// ifNotExists is never asked, its flag the only way to set it; given, it stands in the trail with its flag.
func ifNotExists(given bool) domain.Confirm {
	if given {
		return domain.Confirm{Answer: domain.Answer[bool]{Value: true, Origin: domain.OriginRequest}}
	}

	return domain.Confirm{Answer: domain.Answer[bool]{Origin: domain.OriginDefault}, Skip: "set by its flag only"}
}

// ----------------------------------------------------------------------------
// Resolve and validation

// Resolve settles the request on the facts, every field answered as NewQuestions answers it; a request with problems is refused with all of them, by domain.Invalid.
func Resolve(f Facts, req Request) (Request, error) {
	questions, err := resolve(f, req)
	return questions.Request(), err
}

// resolve is the questions of the request on the facts, refused when one is not settled.
func resolve(f Facts, req Request) (Questions, error) {
	questions := NewQuestions(f, req)
	if len(questions.Errors) > 0 {
		return Questions{}, domain.Invalid(questions.Errors)
	}

	return questions, nil
}

// Request is the request the questions answer.
func (q Questions) Request() Request {
	return Request{
		Branches:    q.Branches.Given,
		From:        q.From.Value,
		Given:       q.Questions.Given(),
		IfNotExists: q.IfNotExists.Value,
	}
}

func (f Facts) problems(q Questions) []domain.FieldError {
	problems := f.branchProblems(q)
	if from := q.From.Value; from != "" && !f.Candidate(from) {
		problems = append(problems, domain.FieldError{Path: pathFrom, Code: domain.CodeNotFound, Template: "{path}: {value} not found", Params: domain.Params{"value": from}})
	}

	problems = append(problems, q.Questions.Problems()...)
	return append(problems, f.crossProblems(q)...)
}

// branchProblems are the refusals of the branches given, each as a field error of its branch.
func (f Facts) branchProblems(q Questions) []domain.FieldError {
	branches := q.Branches.Given
	if len(branches) == 0 {
		return []domain.FieldError{domain.Required(pathBranches)}
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
			Path:     pathFrom,
			Code:     domain.CodeRequired,
			Template: "{path} is required: a branch that already exists locally needs its parent named (it can't be inferred, and `wtm sync` needs it)",
			Params:   domain.Params{"because": "create.parent_required"},
		})
	}

	if from != "" && slices.Contains(branches, from) {
		problems = append(problems, domain.FieldError{Path: pathFrom, Code: domain.CodeSelfParent, Template: "{value} cannot be its own parent: pass another value to {path}", Params: domain.Params{"value": from}, With: []string{pathBranches}})
	}

	return problems
}

func branchPath(index int) string {
	return pathBranches + "[" + strconv.Itoa(index) + "]"
}
