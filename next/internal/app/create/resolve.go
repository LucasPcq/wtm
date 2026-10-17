package create

import (
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/project"
	"github.com/LucasPcq/wtm/next/internal/wtm/run"
)

const (
	codeRequired = "required"
	codeOneOf    = "one_of"
)

var rememberable = []string{"env_from", "isolation", "fast_forward"}

func Resolve(f Facts, req Request) (Request, error) {
	questions := Form(f, req)
	if len(questions.Errors) > 0 {
		return Request{}, wtm.Invalid(questions.Errors)
	}
	return questions.Request(), nil
}

func (q Questions) Request() Request {
	return Request{
		Branches:    q.Branches.Given,
		From:        q.From.Value,
		EnvFrom:     q.EnvFrom.Value,
		Isolation:   q.Isolation.Value,
		FastForward: new(q.FastForward.Value),
		IfNotExists: q.IfNotExists,
		Remember:    q.Remember,
	}
}

func (f Facts) problems(q Questions) []wtm.FieldError {
	problems := f.branchProblems(q)
	if from := q.From.Value; from != "" && !f.candidate(from) {
		problems = append(problems, wtm.FieldError{Path: "from", Code: "not_found", Message: "{path}: " + from + " not found", Params: wtm.Params{"value": from}})
	}
	problems = append(problems, oneOf(choice{Path: "env_from", Value: string(q.EnvFrom.Value), Accepted: names(project.EnvStrategies())})...)
	if q.Isolation.Origin == OriginRequest {
		problems = append(problems, oneOf(choice{Path: "isolation", Value: string(q.Isolation.Value), Accepted: names(run.Isolations())})...)
	}
	for index, id := range q.Remember {
		problems = append(problems, oneOf(choice{Path: "remember[" + strconv.Itoa(index) + "]", Value: id, Accepted: rememberable})...)
	}
	return append(problems, f.crossProblems(q)...)
}

func (f Facts) branchProblems(q Questions) []wtm.FieldError {
	branches := q.Branches.Given
	if len(branches) == 0 {
		return []wtm.FieldError{{Path: "branches", Code: codeRequired, Message: "{path} is required"}}
	}
	var problems []wtm.FieldError
	for index, d := range f.decisions(Request{Branches: branches, IfNotExists: q.IfNotExists}) {
		refused, isRefused := d.(refusal)
		if !isRefused || slices.Contains(branches[:index], branches[index]) {
			continue
		}
		problems = append(problems, wtm.FieldError{Path: branchPath(index), Code: refused.Code, Message: refused.Message, Params: refused.Params})
	}
	return problems
}

func (f Facts) crossProblems(q Questions) []wtm.FieldError {
	var problems []wtm.FieldError
	branches, from := q.Branches.Given, q.From.Value
	for index, name := range branches {
		if name != "" && slices.Contains(branches[:index], name) {
			problems = append(problems, wtm.FieldError{Path: branchPath(index), Code: "distinct", Message: "{path}: " + name + " is given twice", Params: wtm.Params{"value": name}})
		}
	}
	if from == "" && slices.ContainsFunc(branches, f.existing) {
		problems = append(problems, wtm.FieldError{
			Path:    "from",
			Code:    codeRequired,
			Message: "{path} is required: a branch that already exists locally needs its parent named (it can't be inferred, and `wtm sync` needs it)",
			Params:  wtm.Params{"because": "create.parent_required"},
		})
	}
	if from != "" && slices.Contains(branches, from) {
		problems = append(problems, wtm.FieldError{Path: "from", Code: "self_parent", Message: from + " cannot be its own parent: pass another value to {path}", Params: wtm.Params{"value": from}})
	}
	return problems
}

type choice struct {
	Path     string
	Value    string
	Accepted []string
}

func oneOf(c choice) []wtm.FieldError {
	if c.Value == "" || slices.Contains(c.Accepted, c.Value) {
		return nil
	}
	return []wtm.FieldError{{
		Path:     c.Path,
		Code:     codeOneOf,
		Message:  "{path} must be one of " + strings.Join(c.Accepted, ", ") + ", not " + c.Value,
		Params:   wtm.Params{"value": c.Value},
		Accepted: c.Accepted,
	}}
}

func names[T ~string](values []T) []string {
	found := make([]string, 0, len(values))
	for _, value := range values {
		found = append(found, string(value))
	}
	return found
}

func branchPath(index int) string {
	return "branches[" + strconv.Itoa(index) + "]"
}
