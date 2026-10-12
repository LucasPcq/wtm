package render

import (
	"fmt"
	"io"
	"strings"

	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/kernel/text"
)

const (
	causeSeparator = ": "
	fieldSeparator = "; "
	liftedByFmt    = "%s (%s)"
	detailLineFmt  = "%s%s%s\n"
)

type FailureParams struct {
	Err   kernel.Error
	Names Names
	Next  []NextStepParams
}

// Failure is the report of a run that did not apply: the error's sentence,
// then what it names one by one — each field to fix, each refusal and what
// lifts it.
func Failure(w io.Writer, params FailureParams) {
	Error(w, ErrorMessage(params.Err))
	for _, line := range failureDetails(params.Err, params.Names) {
		fmt.Fprintf(w, detailLineFmt, Indent, Indent, line)
	}
	NextSteps(w, params.Next)
}

// FailureLine is the failure on one line, for a run whose report --quiet threw
// away.
func FailureLine(err kernel.Error, names Names) string {
	details := failureDetails(err, names)
	if len(details) == 0 {
		return ErrorMessage(err)
	}
	return ErrorMessage(err) + causeSeparator + strings.Join(details, fieldSeparator)
}

func failureDetails(err kernel.Error, names Names) []string {
	switch typed := err.(type) {
	case *kernel.InvalidError:
		lines := make([]string, 0, len(typed.Fields))
		for _, field := range typed.Fields {
			lines = append(lines, FieldMessage(field, names))
		}
		return lines
	case *kernel.RefusedError:
		lines := make([]string, 0, len(typed.Blockers))
		for _, blocker := range typed.Blockers {
			lines = append(lines, blockerLine(blocker, names))
		}
		return lines
	case *kernel.Failure:
		return nil
	}
	return nil
}

// ErrorMessage is the error's sentence from the catalogue. An error the engine
// could not name more precisely — internal, or invalid with no field to point
// at — keeps its cause: it is all the reader has to go on.
func ErrorMessage(err kernel.Error) string {
	base := err.Base()
	message := text.Message(base.Code, base.Params)
	if base.Cause == nil || !keepsCause(err) {
		return message
	}
	return message + causeSeparator + base.Cause.Error()
}

func keepsCause(err kernel.Error) bool {
	switch typed := err.(type) {
	case *kernel.InvalidError:
		return len(typed.Fields) == 0
	case *kernel.RefusedError:
		return false
	case *kernel.Failure:
		return typed.Kind() == kernel.KindInternal
	}
	return false
}

// FieldMessage names the field, and the fields it is checked against, the way
// the surface spells them.
func FieldMessage(field kernel.FieldError, names Names) string {
	spelled := field
	spelled.Path = names(field.Path)
	spelled.With = make([]string, 0, len(field.With))
	for _, path := range field.With {
		spelled.With = append(spelled.With, names(path))
	}
	return text.Field(spelled)
}

func BlockerMessage(blocker kernel.Blocker) string {
	return text.Message(blocker.Code, blocker.Params)
}

func blockerLine(blocker kernel.Blocker, names Names) string {
	if blocker.Field == "" {
		return BlockerMessage(blocker)
	}
	return fmt.Sprintf(liftedByFmt, BlockerMessage(blocker), names(blocker.Field))
}

func WarningMessage(warning kernel.Warning) string {
	return text.Message(warning.Code, warning.Params)
}
