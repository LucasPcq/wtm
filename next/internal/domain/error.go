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

// The suffixes of the codes every command refuses a repository's config with, after its name: <command>.config_not_found when the repository has no wtm config, <command>.config_invalid when its config.toml does not parse or validate. Each surface reads the refusal from the suffix (the CLI exits 12 and 1).
const (
	ConfigNotFound = ".config_not_found"
	ConfigInvalid  = ".config_invalid"
)

// The suffixes of the codes every read says what it could not read with, after its name: <command>.unreadable fails a subject it could not read, <command>.meta_unreadable warns of a meta record that exists but does not read, <command>.jobs_unknown of a run daemon that could not tell which jobs are up, <command>.prs_unavailable of a forge that could not be asked.
const (
	Unreadable     = ".unreadable"
	MetaUnreadable = ".meta_unreadable"
	JobsUnknown    = ".jobs_unknown"
	DaemonSkew     = ".daemon_skew"
	PRsUnavailable = ".prs_unavailable"
)

// MetaNewer is the suffix, after a command's name, of the code that refuses to write a meta record a newer wtm wrote (exit 22): clean, prune and reparent refuse a request that would delete or rewrite one.
const MetaNewer = ".meta_newer"

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
	Accepted []string  // the values the field accepts, when they are a closed set
	With     []string  // the other fields the problem involves
	FollowUp *FollowUp // a command that gets past the problem, if any
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

// CommandGo is the command a follow-up enters a worktree with: go, the shell function wtm shell-init installs, which cds into the path resolve prints; answered as resolve is, by PathWorktree.
const CommandGo = "go"

// The vocabulary a follow-up names a command with, for the commands whose follow-ups other commands give: clean removes a worktree, prune one done with, reparent changes the parent it is rebased onto, env settles its .env files, fast-forward brings its branch up to origin's, sync rebases it onto its parent, exec runs a command in it, run-up starts its jobs, each answered by PathWorktree; init sets wtm up in a repository.
const (
	CommandClean       = "clean"
	CommandPrune       = "prune"
	CommandReparent    = "reparent"
	CommandEnv         = "env"
	CommandFastForward = "fast-forward"
	CommandSync        = "sync"
	CommandExec        = "exec"
	CommandInit        = "init"
	CommandRunUp       = "run-up"
	PathWorktree       = "worktree" // the worktree a follow-up acts on: its branch or, on no branch, its path
	PathTo             = "to"       // the branch reparent moves a worktree onto
)

// Enter is the follow-up that enters worktree, by its branch or, on no branch, its path; its caller gives it a code.
func Enter(worktree string) FollowUp {
	return FollowUp{Command: CommandGo, Answers: []Answered{{Path: PathWorktree, Values: []string{worktree}}}, Message: "jump in"}
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
	CodeAmbiguous  = "ambiguous"   // the value names several things the repository holds
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
	return fmt.Sprintf("%s is at schema %d, newer than the %d this wtm writes: upgrade wtm to use it", e.Path, e.Found, e.Supported)
}

// MetaNewerParams are the code of a refusal, what it leaves undone ("nothing removed"), and the worktrees it refuses with the record a newer wtm wrote of each, in the same order.
type MetaNewerParams struct {
	Code      string
	Undone    string
	Worktrees []string
	Records   []*SchemaNewer
}

// MetaNewerRefusal refuses a request that would write the record of each of p.Worktrees, which a newer wtm wrote: KindPrecondition, caused by each record's *SchemaNewer, which the CLI exits 22 on; nil when p names none.
func MetaNewerRefusal(p MetaNewerParams) *Error {
	if len(p.Worktrees) == 0 {
		return nil
	}

	causes := make([]string, 0, len(p.Records))
	errs := make([]error, 0, len(p.Records))
	for _, record := range p.Records {
		causes, errs = append(causes, record.Error()), append(errs, record)
	}
	names := strings.Join(p.Worktrees, ", ")
	return &Error{Kind: KindPrecondition, Code: p.Code, Message: p.Undone + ": a newer wtm wrote the record of " + names + ": " + strings.Join(causes, "; "), Params: Params{"worktrees": names}, Cause: errors.Join(errs...)}
}
