// Package wtm is the domain of wtm-next: its nouns, errors, warnings and events, pure and stdlib only.
package wtm

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
	KindPrecondition Kind = "precondition"
	KindRefused      Kind = "refused"
	KindCancelled    Kind = "cancelled"
	KindInternal     Kind = "internal"
)

const (
	CodeInvalidRequest = "request.invalid"
	CodeInterrupted    = "unit.interrupted"
	CodeInternal       = "internal"
)

type Params map[string]string

type Error struct {
	Kind     Kind
	Code     string
	Message  string
	Params   Params
	Fields   []FieldError
	FollowUp *FollowUp
	Cause    error
}

type FieldError struct {
	Path     string
	Code     string
	Message  string
	Params   Params
	Accepted []string
}

type FollowUp struct {
	Command string
	Request json.RawMessage
	Code    string
	Message string
	Params  Params
}

func (e *Error) Error() string {
	if e.Cause == nil {
		return e.Message
	}
	return e.Message + ": " + e.Cause.Error()
}

func (e *Error) Unwrap() error { return e.Cause }

func Invalid(fields []FieldError) *Error {
	return &Error{Kind: KindInvalid, Code: CodeInvalidRequest, Message: "the request is invalid", Fields: fields}
}

func AsError(err error) *Error {
	if err == nil {
		return nil
	}
	var known *Error
	if errors.As(err, &known) {
		return known
	}
	if errors.Is(err, context.Canceled) {
		return &Error{Kind: KindCancelled, Code: CodeInterrupted, Message: "interrupted", Cause: err}
	}
	return &Error{Kind: KindInternal, Code: CodeInternal, Message: "internal error", Cause: err}
}
