package domain

import "slices"

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

// Confirm is a yes-or-no question: its answer, the reason it is not asked (Skip, empty when it is), and the answer remembered for it (nil when none).
type Confirm struct {
	Answer[bool]
	Skip       string
	Remembered *bool
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
