package domain

import (
	"slices"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

// Origin says where an answer to a question came from.
type Origin string

// The origins of an answer.
const (
	OriginRequest    Origin = "request"    // a flag or the request
	OriginRemembered Origin = "remembered" // the answer remembered in config.toml
	OriginConfig     Origin = "config"     // a value of the project's configuration
	OriginDefault    Origin = "default"    // the question's default
)

// Answer is the value a question settled on, and where that value came from.
type Answer[T any] struct {
	Value  T
	Origin Origin
}

// Option is one value a choice offers, how it reads (Label, empty when it reads as its value), a badge the wizard shows beside it (`on origin`, a count of changes), and how a trail names it once picked (Short, empty when by its value).
type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Badge string `json:"badge,omitempty"`
	Short string `json:"short,omitempty"`
}

// Choice is a question with a closed set of options: its answer, its options, the reason it is not asked (Skip, empty when it is), and the answer remembered for it.
type Choice[T ~string] struct {
	Answer[T]
	Options    []T
	Skip       string
	Remembered T
}

// Confirm is a yes-or-no question: its answer and the reason it is not asked (Skip, empty when it is).
type Confirm struct {
	Answer[bool]
	Skip string
}

// IsolationHelp is the help of the isolation question, the same in every command that asks it.
const IsolationHelp = "How the new worktree stands against its source: isolated (its own ports, compose project and namespaces in shared services, in the .env and at run time) or verbatim (.env kept exactly as copied, run on its source's ports and data); defaults to run.toml's isolation, else isolated"

// IsolationAnswer is the isolation a question the request leaves unanswered settles on: the remembered one (empty when none) while run.toml declares something a worktree isolates, else run.toml's, else isolated.
func IsolationAnswer(remembered project.Isolation, run project.Run) Answer[project.Isolation] {
	switch {
	case remembered != "" && run.IsolationApplies():
		return Answer[project.Isolation]{Value: remembered, Origin: OriginRemembered}
	case run.Isolation != "":
		return Answer[project.Isolation]{Value: run.Isolation.Effective(), Origin: OriginConfig}
	}

	return Answer[project.Isolation]{Value: project.Isolated, Origin: OriginDefault}
}

// FastForwardParams is what the question whether to fast-forward the source first settles from: the request's answer, why the question is skipped, and the remembered source update.
type FastForwardParams struct {
	Given      *bool  // nil when the request does not say
	Skip       string // why the question is not asked, empty when it is
	Remembered string // project.SourceUpdateFastForward or project.SourceUpdateKeep, empty when none
}

// FastForwardAnswer is the question whether to fast-forward the source first: the request's answer, none while the question is skipped, else the remembered one, else no.
func FastForwardAnswer(params FastForwardParams) Confirm {
	question := Confirm{Skip: params.Skip}
	switch {
	case params.Given != nil:
		question.Answer = Answer[bool]{Value: *params.Given, Origin: OriginRequest}
	case params.Skip != "":
	case params.Remembered != "":
		question.Answer = Answer[bool]{Value: params.Remembered == project.SourceUpdateFastForward, Origin: OriginRemembered}
	default:
		question.Answer = Answer[bool]{Origin: OriginDefault}
	}

	return question
}

// CodeOneOf is the field-error code of a value that is not one of those its field accepts.
const CodeOneOf = "one_of"

// OneOfParams is a value given to a field with a closed set of accepted values.
type OneOfParams struct {
	Path     string
	Value    string
	Accepted []string
}

// OneOf is the problem of a value that is not one of those accepted, as a field error; an empty value is none.
func OneOf(params OneOfParams) []FieldError {
	if params.Value == "" || slices.Contains(params.Accepted, params.Value) {
		return nil
	}

	return []FieldError{{
		Path:     params.Path,
		Code:     CodeOneOf,
		Template: "{path} must be one of {accepted}, not {value}",
		Params:   Params{"value": params.Value},
		Accepted: params.Accepted,
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

// Options offers each of values as an option that reads as its value, in order.
func Options[T ~string](values []T) []Option {
	found := make([]Option, 0, len(values))
	for _, value := range values {
		found = append(found, Option{Value: string(value)})
	}

	return found
}
