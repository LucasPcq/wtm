package output

import (
	"io"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

type FailureParams struct {
	Err   *domain.Error
	Next  []NextStep
	Flags Flags
}

func Failure(w io.Writer, params FailureParams) {
	ErrorLine(w, ErrorText(params.Err))
	for _, line := range fieldLines(params) {
		Message(w, Indent+line)
	}
	NextSteps(w, params.Next)
}

func QuietFailure(w io.Writer, params FailureParams) {
	ErrorLine(w, failureLine(params))
}

func failureLine(params FailureParams) string {
	fields := fieldLines(params)
	if len(fields) == 0 {
		return ErrorText(params.Err)
	}
	return ErrorText(params.Err) + ": " + strings.Join(fields, "; ")
}

func fieldLines(params FailureParams) []string {
	lines := make([]string, 0, len(params.Err.Fields))
	for _, field := range params.Err.Fields {
		lines = append(lines, field.Message(params.Flags.Spell(field.Path)))
	}
	return lines
}

func ErrorText(err *domain.Error) string {
	keepsCause := err.Kind == domain.KindInternal || err.Kind == domain.KindInvalid && len(err.Fields) == 0
	if err.Cause == nil || !keepsCause {
		return err.Message
	}
	return err.Message + ": " + err.Cause.Error()
}

var reasonLabels = map[domain.Reason]string{
	domain.ReasonUnchanged:   "unchanged",
	domain.ReasonIncomplete:  "left incomplete",
	domain.ReasonInterrupted: "interrupted",
	domain.ReasonNotReached:  "not reached",
}

type Unfinished struct {
	Subject string
	Status  domain.Status
	Reason  domain.Reason
	Err     *domain.Error
}

func UnfinishedLine(w io.Writer, item Unfinished) {
	line := item.Subject + "  " + unfinishedWhy(item)
	if item.Status == domain.StatusFailed {
		ErrorLine(w, line)
		return
	}
	WarningLine(w, line)
}

func unfinishedWhy(item Unfinished) string {
	if item.Err == nil {
		return reasonLabels[item.Reason]
	}
	why := ErrorText(item.Err)
	if item.Reason != domain.ReasonIncomplete {
		return why
	}
	return why + Dot + reasonLabels[item.Reason]
}
