package output

import (
	"io"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/wtm"
)

type FailureParams struct {
	Err  *wtm.Error
	Next []NextStep
}

func Failure(w io.Writer, params FailureParams) {
	errorLine(w, errorText(params.Err))
	for _, line := range fieldLines(params.Err) {
		message(w, Indent+line)
	}
	nextSteps(w, params.Next)
}

func QuietFailure(w io.Writer, err *wtm.Error) {
	errorLine(w, failureLine(err))
}

func failureLine(err *wtm.Error) string {
	fields := fieldLines(err)
	if len(fields) == 0 {
		return errorText(err)
	}
	return errorText(err) + ": " + strings.Join(fields, "; ")
}

func fieldLines(err *wtm.Error) []string {
	lines := make([]string, 0, len(err.Fields))
	for _, field := range err.Fields {
		lines = append(lines, field.Message(Flag(field.Path)))
	}
	return lines
}

func errorText(err *wtm.Error) string {
	keepsCause := err.Kind == wtm.KindInternal || err.Kind == wtm.KindInvalid && len(err.Fields) == 0
	if err.Cause == nil || !keepsCause {
		return err.Message
	}
	return err.Message + ": " + err.Cause.Error()
}
