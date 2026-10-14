// Package text turns a code of the engine and its params into English, from the catalogue of the registry served.
package text

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/kernel"
)

const (
	paramAccepted = "accepted"
	paramWith     = "with"
	duplicateFmt  = "text: code %q has two messages"
)

const (
	codeStatusDone        kernel.Code = "status.done"
	codeStatusFailed      kernel.Code = "status.failed"
	codeStatusSkipped     kernel.Code = "status.skipped"
	codeStatusCancelled   kernel.Code = "status.cancelled"
	codeReasonInterrupted kernel.Code = "reason.interrupted"
	codeReasonNotReached  kernel.Code = "reason.not_reached"
	codeReasonUnchanged   kernel.Code = "reason.unchanged"
	codeReasonUnsafe      kernel.Code = "reason.unsafe"
	codeReasonLocked      kernel.Code = "reason.locked"
	codeReasonIncomplete  kernel.Code = "reason.incomplete"
)

// Messages are the templates of the codes one package declares, each filled
// by its {param}.
type Messages map[kernel.Code]string

var kernelMessages = Messages{
	kernel.CodeRequired:      "{path} is required",
	kernel.CodeRequiredOneOf: "one of {path}, {with} is required",
	kernel.CodeInvalid:       "{path} is invalid",
	kernel.CodeNotFound:      "{path}: {value} not found",
	kernel.CodeOneOf:         "{path} must be one of {accepted}, not {value}",
	kernel.CodeExclusiveWith: "{path} cannot be used with {with}",
	kernel.CodeRequires:      "{path} requires {with}",
	kernel.CodeDistinct:      "{path}: {value} is given twice",
	kernel.CodeSelfParent:    "{value} cannot be its own parent: pass another value to {path}",
	kernel.CodeTooShort:      "{path} must be at least {limit} characters",
	kernel.CodeTooLong:       "{path} must be at most {limit} characters",
	kernel.CodeTooFew:        "{path} needs at least {limit} entries",
	kernel.CodeTooMany:       "{path} takes at most {limit} entries",
	kernel.CodePattern:       "{path}: {value} does not match {pattern}",
	kernel.CodeUnknownField:  "{path} is not a field",

	kernel.CodeInvalidRequest: "the request is invalid",
	kernel.CodeInternal:       "internal error",
	kernel.CodeInterrupted:    "interrupted",
	kernel.CodeStepFailed:     "{step} failed",
	kernel.CodeUndoFailed:     "could not undo {step}, left behind: {left}",
	kernel.CodePhaseFailed:    "{phase} failed",

	kernel.CodeLockHeld:         "{key} is held by {command} (pid {pid})",
	kernel.CodeLocksUnavailable: "ran without operation locks",
	kernel.CodePlanStale:        "the facts changed since the preview",
	kernel.CodeStateSchemaNewer: "{path} is at schema {found}, this wtm supports {supported}",

	codeStatusDone:        "done",
	codeStatusFailed:      "failed",
	codeStatusSkipped:     "skipped",
	codeStatusCancelled:   "cancelled",
	codeReasonInterrupted: "interrupted",
	codeReasonNotReached:  "not reached",
	codeReasonUnchanged:   "unchanged",
	codeReasonUnsafe:      "unsafe",
	codeReasonLocked:      "locked",
	codeReasonIncomplete:  "incomplete",
}

var statusCodes = map[kernel.Status]kernel.Code{
	kernel.StatusDone:      codeStatusDone,
	kernel.StatusFailed:    codeStatusFailed,
	kernel.StatusSkipped:   codeStatusSkipped,
	kernel.StatusCancelled: codeStatusCancelled,
}

var reasonCodes = map[kernel.Reason]kernel.Code{
	kernel.ReasonInterrupted: codeReasonInterrupted,
	kernel.ReasonNotReached:  codeReasonNotReached,
	kernel.ReasonUnchanged:   codeReasonUnchanged,
	kernel.ReasonUnsafe:      codeReasonUnsafe,
	kernel.ReasonLocked:      codeReasonLocked,
	kernel.ReasonIncomplete:  codeReasonIncomplete,
}

// Catalog is the kernel's messages and those of every entry a surface serves.
type Catalog struct {
	messages Messages
}

// NewCatalog panics on a code two sets spell: a code means one thing, and the
// registry is built at startup, before anything runs.
func NewCatalog(sets ...Messages) Catalog {
	merged := Messages{}
	for _, set := range append([]Messages{kernelMessages}, sets...) {
		for code, template := range set {
			if _, taken := merged[code]; taken {
				panic(fmt.Sprintf(duplicateFmt, code))
			}
			merged[code] = template
		}
	}
	return Catalog{messages: merged}
}

// Message renders code over params; a code missing from the catalogue reads
// as itself, which the catalog rule of archlint forbids.
func (c Catalog) Message(code kernel.Code, params kernel.Params) string {
	template, known := c.messages[code]
	if !known {
		return string(code)
	}
	pairs := make([]string, 0, 2*len(params))
	for key, value := range params {
		pairs = append(pairs, "{"+key+"}", value)
	}
	return strings.NewReplacer(pairs...).Replace(template)
}

// Field renders a field error, with why it is required when a rule says so.
func (c Catalog) Field(problem kernel.FieldError) string {
	params := kernel.Params{
		kernel.ParamPath: problem.Path,
		paramAccepted:    strings.Join(problem.Accepted, ", "),
		paramWith:        strings.Join(problem.With, ", "),
	}
	for key, value := range problem.Params {
		params[key] = value
	}
	message := c.Message(problem.Code, params)
	because := problem.Params[kernel.ParamBecause]
	if because == "" {
		return message
	}
	return message + ": " + c.Message(kernel.Code(because), params)
}

func (c Catalog) Status(status kernel.Status) string {
	code, known := statusCodes[status]
	if !known {
		return string(status)
	}
	return c.Message(code, nil)
}

func (c Catalog) Reason(reason kernel.Reason) string {
	code, known := reasonCodes[reason]
	if !known {
		return string(reason)
	}
	return c.Message(code, nil)
}
