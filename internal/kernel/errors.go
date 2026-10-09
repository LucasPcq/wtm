// Package kernel is the pure contract every command, dispatch and surface share.
package kernel

import (
	"context"
	"encoding/json"
	"errors"
)

// Code names a fact the engine reports; kernel/text turns it into a message.
type Code string

// Params are what a message needs, by name: {"branch": "feat/x"}.
type Params map[string]string

type Kind string

const (
	KindInvalid      Kind = "invalid"
	KindNotFound     Kind = "not_found"
	KindConflict     Kind = "conflict"
	KindRefused      Kind = "refused"
	KindPrecondition Kind = "precondition"
	KindCancelled    Kind = "cancelled"
	KindInternal     Kind = "internal"
)

const (
	CodeRequired      Code = "required"
	CodeRequiredOneOf Code = "required_one_of"
	CodeInvalid       Code = "invalid"
	CodeNotFound      Code = "not_found"
	CodeOneOf         Code = "one_of"
	CodeExclusiveWith Code = "exclusive_with"
	CodeRequires      Code = "requires"
	CodeDistinct      Code = "distinct"
	CodeSelfParent    Code = "self_parent"
	CodeTooShort      Code = "too_short"
	CodeTooLong       Code = "too_long"
	CodeTooFew        Code = "too_few"
	CodeTooMany       Code = "too_many"
	CodePattern       Code = "pattern"

	CodeInvalidRequest Code = "request.invalid"
	CodeInternal       Code = "internal"
	CodeInterrupted    Code = "unit.interrupted"
	CodeStepFailed     Code = "unit.step_failed"
	CodeUndoFailed     Code = "unit.undo_failed"
	CodePhaseFailed    Code = "unit.phase_failed"
)

const (
	ParamPath    = "path"
	ParamValue   = "value"
	ParamLimit   = "limit"
	ParamPattern = "pattern"
	ParamBecause = "because"
	ParamStep    = "step"
	ParamPhase   = "phase"
	ParamLeft    = "left"
	ParamStatus  = "status"
)

// Problem is what every error carries, whatever its kind.
type Problem struct {
	Code     Code      `json:"code"`
	Params   Params    `json:"params,omitempty"`
	FollowUp *FollowUp `json:"follow_up,omitempty"`
	Cause    error     `json:"-"`
}

//go-sumtype:decl Error

// Error is one of InvalidError, RefusedError or Failure: the kind is the
// type, so an invalid request always names its fields and a refusal its
// blockers.
type Error interface {
	error
	Kind() Kind
	Base() *Problem
	sealed()
}

// InvalidError is the 422: the request breaks its fields' constraints or rules.
type InvalidError struct {
	Problem
	Fields []FieldError `json:"fields"`
}

// RefusedError is the 403: each refusal, and the field that lifts it.
type RefusedError struct {
	Problem
	Blockers []Blocker `json:"blockers"`
}

// Failure is every other kind: not found, conflict, precondition, cancelled,
// internal. Only its constructors set the kind.
type Failure struct {
	Problem
	kind Kind
}

type FieldError struct {
	Path     string   `json:"path"`
	Code     Code     `json:"code"`
	Params   Params   `json:"params,omitempty"`
	Accepted []string `json:"accepted,omitempty"`
	With     []string `json:"with,omitempty"`
}

type Blocker struct {
	Code   Code   `json:"code"`
	Params Params `json:"params,omitempty"`
	Field  string `json:"field,omitempty"`
}

// Error carries the code, never a sentence: the text is kernel/text's.
func (p *Problem) Error() string {
	if p.Cause == nil {
		return string(p.Code)
	}
	return string(p.Code) + ": " + p.Cause.Error()
}

func (p *Problem) Unwrap() error { return p.Cause }

func (p *Problem) Base() *Problem { return p }

func (*InvalidError) Kind() Kind { return KindInvalid }
func (*RefusedError) Kind() Kind { return KindRefused }
func (f *Failure) Kind() Kind    { return f.kind }

func (*InvalidError) sealed() {}
func (*RefusedError) sealed() {}
func (*Failure) sealed()      {}

// Invalid is the 422 naming every field at fault.
func Invalid(fields []FieldError) *InvalidError {
	return &InvalidError{Problem: Problem{Code: CodeInvalidRequest}, Fields: fields}
}

func NotFound(problem Problem) *Failure { return &Failure{Problem: problem, kind: KindNotFound} }
func Conflict(problem Problem) *Failure { return &Failure{Problem: problem, kind: KindConflict} }
func Precondition(problem Problem) *Failure {
	return &Failure{Problem: problem, kind: KindPrecondition}
}
func Cancelled(problem Problem) *Failure { return &Failure{Problem: problem, kind: KindCancelled} }
func Internal(problem Problem) *Failure  { return &Failure{Problem: problem, kind: KindInternal} }

// The JSON of an error names its kind, which the Go type carries.

func (e *InvalidError) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind Kind `json:"kind"`
		Problem
		Fields []FieldError `json:"fields"`
	}{KindInvalid, e.Problem, e.Fields})
}

func (e *RefusedError) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind Kind `json:"kind"`
		Problem
		Blockers []Blocker `json:"blockers"`
	}{KindRefused, e.Problem, e.Blockers})
}

func (f *Failure) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind Kind `json:"kind"`
		Problem
	}{f.kind, f.Problem})
}

type ClassifyParams struct {
	Err    error
	Code   Code
	Params Params
}

// Classify keeps an Error as it is, reads a cancellation as cancelled, and
// files anything else as internal under the code the caller gives.
func Classify(params ClassifyParams) Error {
	var known Error
	if errors.As(params.Err, &known) {
		return known
	}
	if errors.Is(params.Err, context.Canceled) {
		return Cancelled(Problem{Code: CodeInterrupted, Params: params.Params, Cause: params.Err})
	}
	code := params.Code
	if code == "" {
		code = CodeInternal
	}
	return Internal(Problem{Code: code, Params: params.Params, Cause: params.Err})
}
