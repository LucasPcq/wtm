package output

import (
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/domain"
)

const (
	commandName = "create"
)

var reasonLabels = map[domain.Reason]string{
	domain.ReasonUnchanged:   "unchanged",
	domain.ReasonIncomplete:  "left incomplete",
	domain.ReasonInterrupted: "interrupted",
	domain.ReasonNotReached:  "not reached",
}

type OutcomeParams struct {
	Outcome   create.Outcome
	FollowUps []NextStep
	Cwd       string
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
		path := shownPath(shown{Cwd: params.Cwd, Path: item.Detail.Path})
		rows = append(rows, planRow{Label: item.Subject, Values: []string{path + itemNote(item)}})
		anomalies = append(anomalies, anomaliesOf(item)...)
		next = append(next, NextStep{Command: "wtm go " + item.Subject})
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
		drawPlan(w, planParams{Rows: rows, Warnings: anomalies})
	}
	if len(rows) == 0 && len(anomalies) > 0 {
		blank(w)
		for _, text := range anomalies {
			warningLine(w, text)
		}
	}
	unfinished := slices.DeleteFunc(slices.Clone(items), func(item create.Item) bool { return item.Status == domain.StatusDone })
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
	return item.Status == domain.StatusDone || item.Reason == domain.ReasonIncomplete
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
	line := commandName + ": " + strings.Join(parts, dot)
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
	if item.Status == domain.StatusDone && item.Reason == domain.ReasonUnchanged {
		return "unchanged"
	}
	return string(item.Status)
}

func unfinishedLine(w io.Writer, item create.Item) {
	line := item.Subject + "  " + itemWhy(item)
	if item.Status == domain.StatusFailed {
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
	if item.Reason != domain.ReasonIncomplete {
		return why
	}
	return why + dot + reasonLabels[item.Reason]
}
