// Package kernel is the pure contract every command, dispatch and surface share.
package kernel

import (
	"context"
	"errors"
)

// Code names a fact the engine reports; kernel/text turns it into a message.
type Code string

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

type Error struct {
	Kind     Kind              `json:"kind"`
	Code     Code              `json:"code"`
	Params   map[string]string `json:"params,omitempty"`
	Fields   []FieldError      `json:"fields,omitempty"`
	Blockers []Blocker         `json:"blockers,omitempty"`
	FollowUp *FollowUp         `json:"follow_up,omitempty"`
	Cause    error             `json:"-"`
}

// Error carries the code, never a sentence: the text is kernel/text's.
func (e *Error) Error() string {
	if e.Cause == nil {
		return string(e.Code)
	}
	return string(e.Code) + ": " + e.Cause.Error()
}

func (e *Error) Unwrap() error { return e.Cause }

type FieldError struct {
	Path     string            `json:"path"`
	Code     Code              `json:"code"`
	Params   map[string]string `json:"params,omitempty"`
	Accepted []string          `json:"accepted,omitempty"`
	With     []string          `json:"with,omitempty"`
}

// Blocker is one refusal, and the field whose value lifts it (force).
type Blocker struct {
	Code   Code              `json:"code"`
	Params map[string]string `json:"params,omitempty"`
	Field  string            `json:"field,omitempty"`
}

// Invalid is the 422 naming every field at fault.
func Invalid(fields []FieldError) *Error {
	return &Error{Kind: KindInvalid, Code: CodeInvalidRequest, Fields: fields}
}

type ClassifyParams struct {
	Err    error
	Code   Code
	Params map[string]string
}

// Classify keeps an *Error as it is, reads a cancellation as cancelled, and
// files anything else as internal under the code the caller gives.
func Classify(params ClassifyParams) *Error {
	var known *Error
	if errors.As(params.Err, &known) {
		return known
	}
	if errors.Is(params.Err, context.Canceled) {
		return &Error{Kind: KindCancelled, Code: CodeInterrupted, Params: params.Params, Cause: params.Err}
	}
	code := params.Code
	if code == "" {
		code = CodeInternal
	}
	return &Error{Kind: KindInternal, Code: code, Params: params.Params, Cause: params.Err}
}
