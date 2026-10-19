// Package domain is the contract every use case speaks: errors, warnings, events, outcomes, and the operations in progress with the locks they hold.
package domain

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Kind classifies a failure by what the caller can do about it; the command line maps each kind onto an exit code.
type Kind string

// The kinds of failure.
const (
	KindInvalid      Kind = "invalid"      // the request itself is wrong
	KindConflict     Kind = "conflict"     // something already there, or held by another operation, collides with the request
	KindPrecondition Kind = "precondition" // something the request needs is missing (no wtm config)
	KindRefused      Kind = "refused"      // a safety check refused the request
	KindCancelled    Kind = "cancelled"    // the operation was interrupted
	KindInternal     Kind = "internal"     // something failed that the request could not prevent
)

// The codes shared by every command; a command declares its own beside the code that raises them.
const (
	CodeInvalidRequest = "request.invalid"
	CodeInterrupted    = "unit.interrupted"
	CodeInternal       = "internal"
	CodeNotARepository = "git.not_a_repository"
)

// Params holds the named values a message or a code carries, for the JSON envelope and for templates.
type Params map[string]string

// Error is a failure as a use case reports it: its kind, its stable code, a human message, and what the caller can do next.
type Error struct {
	Kind     Kind
	Code     string
	Message  string
	Params   Params
	Fields   []FieldError // the request's problems, one per field, when Kind is KindInvalid
	FollowUp *FollowUp    // a command that repairs what the failure left behind, if any
	Cause    error
}

// FieldError is one problem with one field of a request.
type FieldError struct {
	Path     string // the field, as the request names it (`from`, `remember[0]`)
	Code     string // the problem, stable across versions
	Template string // the message, with {path}, {accepted} and each param as placeholders
	Params   Params
	Accepted []string // the values the field accepts, when they are a closed set
	With     []string // the other fields the problem involves
}

// Message returns the field error's template with its placeholders filled; path is how the caller spells the field (a flag name on the command line), unless Params names a path itself.
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

// FollowUp is what to do next, a failure's repair or an action offered: a wtm command, as the command line spells it after the program's name, with its request as the answers of that command's fields, run as a dry run when DryRun is set; or, with no command, a link to open, URL.
// The command line spells it at print time, each surface its own way.
type FollowUp struct {
	Command string     `json:"command,omitempty"`
	Answers []Answered `json:"answers,omitempty"`
	DryRun  bool       `json:"dry_run,omitempty"`
	URL     string     `json:"url,omitempty"`
	Code    string     `json:"code"`
	Message string     `json:"message"`
	Params  Params     `json:"params,omitempty"`
}

// The vocabulary a follow-up names a command with, for the commands not yet migrated whose follow-ups several commands give: go enters a worktree, answered by PathWorktree.
const (
	CommandGo    = "go"
	PathWorktree = "worktree" // the worktree a follow-up acts on: its branch or, on no branch, its path
)

// Enter is the follow-up that enters worktree, by its branch or, on no branch, its path.
func Enter(worktree string) FollowUp {
	return FollowUp{Command: CommandGo, Answers: []Answered{{Path: PathWorktree, Values: []string{worktree}}}}
}

// Error returns the message, followed by the cause's when there is one.
func (e *Error) Error() string {
	if e == nil {
		return "<nil *wtm.Error>"
	}
	if e.Cause == nil {
		return e.Message
	}

	return e.Message + ": " + e.Cause.Error()
}

// Text returns the text the failure is shown with: its message, followed by its cause only for an internal failure or for an invalid request that names no field, where the cause is all the reader has to go on.
func (e *Error) Text() string {
	keepsCause := e.Kind == KindInternal || e.Kind == KindInvalid && len(e.Fields) == 0
	if e.Cause == nil || !keepsCause {
		return e.Message
	}
	return e.Message + ": " + e.Cause.Error()
}

// Unwrap returns the cause, so errors.Is and errors.As see through the failure.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}

	return e.Cause
}

// Invalid returns the failure of a request whose fields have the given problems.
func Invalid(fields []FieldError) *Error {
	return &Error{Kind: KindInvalid, Code: CodeInvalidRequest, Message: "the request is invalid", Fields: fields}
}

// The field-error codes every command spells the same, with no command prefix.
const (
	CodeRequired   = "required"    // the field has no value
	CodeNotFound   = "not_found"   // the value names nothing the repository holds
	CodeSelfParent = "self_parent" // the branch is given as its own parent
	CodeDistinct   = "distinct"    // the value is given twice
)

// Required returns the problem of the field at path left without a value.
func Required(path string) FieldError {
	return FieldError{Path: path, Code: CodeRequired, Template: "{path} is required"}
}

// InvalidMessage returns the failure of a request that is wrong as a whole, described by message.
func InvalidMessage(message string) *Error {
	return &Error{Kind: KindInvalid, Code: CodeInvalidRequest, Message: message}
}

// AsErrorParams is what AsError turns into a failure: Err, and the code, message and params to give it when it is not one already.
type AsErrorParams struct {
	Err     error
	Code    string
	Message string
	Params  Params
}

// AsError returns params.Err as a failure: the *Error it wraps if any, an interruption if it is a cancelled context, else an internal failure with params.Code and params.Message (or the generic internal code). It returns nil for a nil error.
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

// SchemaNewer is the error of a state file written by a newer wtm, at a schema this one does not know: it refuses to change the file.
type SchemaNewer struct {
	Path      string
	Found     int
	Supported int
}

// Error names the file, its schema and the one this wtm writes, and asks for an upgrade.
func (e *SchemaNewer) Error() string {
	return fmt.Sprintf("%s is at schema %d, newer than the %d this wtm writes: upgrade wtm to change it", e.Path, e.Found, e.Supported)
}
