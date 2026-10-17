package output

import (
	"io"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

type FailureParams struct {
	Err  *domain.Error
	Next []NextStep
}

func Failure(w io.Writer, params FailureParams) {
	errorLine(w, errorText(params.Err))
	for _, line := range fieldLines(params.Err) {
		message(w, Indent+line)
	}
	nextSteps(w, params.Next)
}

func QuietFailure(w io.Writer, err *domain.Error) {
	errorLine(w, failureLine(err))
}

func failureLine(err *domain.Error) string {
	fields := fieldLines(err)
	if len(fields) == 0 {
		return errorText(err)
	}
	return errorText(err) + ": " + strings.Join(fields, "; ")
}

func fieldLines(err *domain.Error) []string {
	lines := make([]string, 0, len(err.Fields))
	for _, field := range err.Fields {
		lines = append(lines, field.Message(Flag(field.Path)))
	}
	return lines
}

func errorText(err *domain.Error) string {
	keepsCause := err.Kind == domain.KindInternal || err.Kind == domain.KindInvalid && len(err.Fields) == 0
	if err.Cause == nil || !keepsCause {
		return err.Message
	}
	return err.Message + ": " + err.Cause.Error()
}
