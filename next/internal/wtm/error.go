// Package wtm is the contract every use case speaks: errors, warnings, events, outcomes, and the operations in progress with the locks they hold.
package wtm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type Kind string

const (
	KindInvalid      Kind = "invalid"
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
	CodeNotARepository = "git.not_a_repository"
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
	Template string
	Params   Params
	Accepted []string
	With     []string
}

func (f FieldError) Message(path string) string {
	pairs := []string{"{accepted}", strings.Join(f.Accepted, ", ")}
	for key, value := range f.Params {
		pairs = append(pairs, "{"+key+"}", value)
	}
	if _, named := f.Params["path"]; !named {
		pairs = append(pairs, "{path}", path)
	}
	return strings.NewReplacer(pairs...).Replace(f.Template)
}

type FollowUp struct {
	Command string
	Request json.RawMessage
	Code    string
	Message string
	Params  Params
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil *wtm.Error>"
	}
	if e.Cause == nil {
		return e.Message
	}
	return e.Message + ": " + e.Cause.Error()
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func Invalid(fields []FieldError) *Error {
	return &Error{Kind: KindInvalid, Code: CodeInvalidRequest, Message: "the request is invalid", Fields: fields}
}

func InvalidMessage(message string) *Error {
	return &Error{Kind: KindInvalid, Code: CodeInvalidRequest, Message: message}
}

type AsErrorParams struct {
	Err     error
	Code    string
	Message string
	Params  Params
}

func AsError(params AsErrorParams) *Error {
	if params.Err == nil {
		return nil
	}
	if known, ok := errors.AsType[*Error](params.Err); ok && known != nil {
		return known
	}
	if errors.Is(params.Err, context.Canceled) {
		return &Error{Kind: KindCancelled, Code: CodeInterrupted, Message: "interrupted", Params: params.Params, Cause: params.Err}
	}
	if params.Code != "" {
		return &Error{Kind: KindInternal, Code: params.Code, Message: params.Message, Params: params.Params, Cause: params.Err}
	}
	return &Error{Kind: KindInternal, Code: CodeInternal, Message: "internal error", Params: params.Params, Cause: params.Err}
}

type SchemaNewer struct {
	Path      string
	Found     int
	Supported int
}

func (e *SchemaNewer) Error() string {
	return fmt.Sprintf("%s is at schema %d, newer than the %d this wtm writes: upgrade wtm to change it", e.Path, e.Found, e.Supported)
}
