package kernel

import (
	"context"
	"encoding/json"
	"errors"
)

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

type Problem struct {
	Code     Code      `json:"code"`
	Params   Params    `json:"params,omitempty"`
	FollowUp *FollowUp `json:"follow_up,omitempty"`
	Cause    error     `json:"-"`
}

//go-sumtype:decl Error

type Error interface {
	error
	Kind() Kind
	Base() *Problem
	sealed()
}

type InvalidError struct {
	Problem
	Fields []FieldError `json:"fields"`
}

type RefusedError struct {
	Problem
	Blockers []Blocker `json:"blockers"`
}

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
