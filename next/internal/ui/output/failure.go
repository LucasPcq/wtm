package output

import (
	"io"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

// FailureParams is a failure to print: the error, the steps that may get past it, and the flags its fields are spelled with.
type FailureParams struct {
	Err   *domain.Error
	Next  []NextStep
	Flags Flags
}

// Failure writes the failure of params to w: its error, a line per field it refuses, then its next steps.
func Failure(w io.Writer, params FailureParams) {
	ErrorLine(w, params.Err.Text())
	for _, line := range fieldLines(params) {
		Message(w, Indent+line)
	}
	NextSteps(w, params.Next)
}

// QuietFailure writes the failure of params to w on one line, its fields joined after the error, as --quiet prints it.
func QuietFailure(w io.Writer, params FailureParams) {
	ErrorLine(w, failureLine(params))
}

func failureLine(params FailureParams) string {
	fields := fieldLines(params)
	if len(fields) == 0 {
		return params.Err.Text()
	}
	return params.Err.Text() + ": " + strings.Join(fields, "; ")
}

func fieldLines(params FailureParams) []string {
	lines := make([]string, 0, len(params.Err.Fields))
	for _, field := range params.Err.Fields {
		lines = append(lines, field.Message(params.Flags.Spell(field.Path)))
	}
	return lines
}

var reasonLabels = map[domain.Reason]string{
	domain.ReasonUnchanged:   "unchanged",
	domain.ReasonIncomplete:  "left incomplete",
	domain.ReasonInterrupted: "interrupted",
	domain.ReasonNotReached:  "not reached",
}

// An Unfinished is an item of an operation that did not end done: the subject it worked on, how and why it stopped.
type Unfinished struct {
	Subject string
	Status  domain.Status
	Reason  domain.Reason
	Err     *domain.Error // the failure that stopped it, if one did
}

// UnfinishedLine writes item to w with why it stopped, as a failure when it failed and as a warning otherwise.
func UnfinishedLine(w io.Writer, item Unfinished) {
	line := item.Subject + "  " + unfinishedWhy(item)
	if item.Status == domain.StatusFailed {
		ErrorLine(w, line)
		return
	}
	WarningLine(w, line)
}

// unfinishedWhy returns the error that stopped item, else its reason; an item left incomplete by an error shows both.
func unfinishedWhy(item Unfinished) string {
	if item.Err == nil {
		return reasonLabels[item.Reason]
	}

	why := item.Err.Text()
	if item.Reason != domain.ReasonIncomplete {
		return why
	}
	return why + Dot + reasonLabels[item.Reason]
}
