package output

import (
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/app"
	"github.com/LucasPcq/wtm/next/internal/app/create"
	"github.com/LucasPcq/wtm/next/internal/wtm"
)

const (
	commandName     = "create"
	reasonSeparator = " · "
	tallySeparator  = " · "
)

var reasonLabels = map[wtm.Reason]string{
	wtm.ReasonUnchanged:   "unchanged",
	wtm.ReasonIncomplete:  "left incomplete",
	wtm.ReasonInterrupted: "interrupted",
	wtm.ReasonNotReached:  "not reached",
}

type OutcomeParams struct {
	Outcome   app.CreateOutcome
	FollowUps []NextStep
}

func Outcome(w io.Writer, params OutcomeParams) {
	items := params.Outcome.Result.Items
	headline(w, items)
	var rows []planRow
	var anomalies []string
	var next []NextStep
	if len(items) > 0 {
		rows = planRowsOf(params.Outcome.Plan)
	}
	for _, item := range items {
		if !standing(item) || item.Detail == nil {
			continue
		}
		view := viewOf(item)
		rows, anomalies, next = append(rows, view.row), append(anomalies, view.anomalies...), append(next, view.next)
	}
	for _, item := range items {
		for _, warned := range item.Warnings {
			anomalies = append(anomalies, item.Subject+"  "+warned.Message)
		}
	}
	for _, warned := range params.Outcome.Warnings {
		anomalies = append(anomalies, warned.Message)
	}
	if len(rows) > 0 {
		blank(w)
		writePlan(w, renderPlan(planParams{Rows: rows, Warnings: anomalies, Width: max(terminalWidth(w)-len(indent), 0)}))
	}
	if len(rows) == 0 && len(anomalies) > 0 {
		blank(w)
		for _, text := range anomalies {
			warningLine(w, text)
		}
	}
	unfinished := slices.DeleteFunc(slices.Clone(items), func(item create.Item) bool { return item.Status == wtm.StatusDone })
	if len(unfinished) > 0 {
		blank(w)
	}
	for _, item := range unfinished {
		unfinishedLine(w, item)
	}
	next = append(next, params.FollowUps...)
	if len(next) > 0 {
		blank(w)
		nextSteps(w, next)
	}
}

func standing(item create.Item) bool {
	return item.Status == wtm.StatusDone || item.Reason == wtm.ReasonIncomplete
}

func headline(w io.Writer, items []create.Item) {
	if len(items) == 0 {
		unchangedLine(w, commandName+": "+nothingToDo)
		return
	}
	count := map[string]int{}
	for _, item := range items {
		count[tallyKey(item)]++
	}
	var parts []string
	for _, key := range []string{"done", "unchanged", "failed", "skipped", "cancelled"} {
		if count[key] > 0 {
			parts = append(parts, strconv.Itoa(count[key])+" "+key)
		}
	}
	line := commandName + ": " + strings.Join(parts, tallySeparator)
	switch {
	case count["failed"] > 0:
		errorLine(w, line)
	case count["skipped"]+count["cancelled"] > 0:
		warningLine(w, line)
	case count["done"] == 0:
		unchangedLine(w, line)
	default:
		successLine(w, line)
	}
}

func tallyKey(item create.Item) string {
	if item.Status == wtm.StatusDone && item.Reason == wtm.ReasonUnchanged {
		return "unchanged"
	}
	return string(item.Status)
}

func unfinishedLine(w io.Writer, item create.Item) {
	line := item.Subject + "  " + itemWhy(item)
	if item.Status == wtm.StatusFailed {
		errorLine(w, line)
		return
	}
	warningLine(w, line)
}

func itemWhy(item create.Item) string {
	if item.Err == nil {
		return reasonLabels[item.Reason]
	}
	why := errorText(item.Err)
	if item.Reason != wtm.ReasonIncomplete {
		return why
	}
	return why + reasonSeparator + reasonLabels[item.Reason]
}

func errorText(err *wtm.Error) string {
	keepsCause := err.Kind == wtm.KindInternal || err.Kind == wtm.KindInvalid && len(err.Fields) == 0
	if err.Cause == nil || !keepsCause {
		return err.Message
	}
	return err.Message + ": " + err.Cause.Error()
}
