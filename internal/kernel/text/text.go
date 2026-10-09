// Package text is the one catalogue that turns a kernel code and its params into English.
package text

import (
	"strings"

	"github.com/LucasPcq/wtm/internal/kernel"
)

const (
	paramAccepted = "accepted"
	paramWith     = "with"
	requiredOneOf = kernel.CodeRequired + ".one_of"
)

var catalog = map[kernel.Code]string{
	kernel.CodeRequired:      "{path} is required",
	requiredOneOf:            "one of {path}, {with} is required",
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

	kernel.CodeInvalidRequest: "the request is invalid",
	kernel.CodeInternal:       "internal error",
	kernel.CodeInterrupted:    "interrupted",
	kernel.CodeStepFailed:     "{step} failed",
	kernel.CodeUndoFailed:     "could not undo {step}, left behind: {left}",
	kernel.CodePhaseFailed:    "{phase} failed",
}

// Message renders code over params; a code missing from the catalogue reads
// as itself, which the catalogue test forbids.
func Message(code kernel.Code, params map[string]string) string {
	template, known := catalog[code]
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
func Field(problem kernel.FieldError) string {
	params := map[string]string{
		kernel.ParamPath: problem.Path,
		paramAccepted:    strings.Join(problem.Accepted, ", "),
		paramWith:        strings.Join(problem.With, ", "),
	}
	for key, value := range problem.Params {
		params[key] = value
	}
	message := Message(fieldCode(problem), params)
	because := problem.Params[kernel.ParamBecause]
	if because == "" {
		return message
	}
	return message + ": " + Message(kernel.Code(because), params)
}

func fieldCode(problem kernel.FieldError) kernel.Code {
	if problem.Code == kernel.CodeRequired && len(problem.With) > 0 {
		return requiredOneOf
	}
	return problem.Code
}
