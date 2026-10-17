package cli

import (
	"cmp"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

const (
	commandName    = "create"
	anomaliesTitle = "Env ports left alone"
)

func planRowsOf(plan create.Plan) []output.PlanRow {
	rows := []output.PlanRow{
		{Label: "Branches", Values: plannedBranches(plan.Branches)},
		{Label: sourceLabel(plan.Branches), Values: []string{plan.From}},
		{Label: "Env", Values: []string{string(plan.EnvFrom)}},
	}
	if plan.Isolation != "" {
		rows = append(rows, output.PlanRow{Label: "Isolation", Values: []string{string(plan.Isolation)}})
	}
	if len(plan.FastForward) > 0 {
		rows = append(rows, output.PlanRow{Label: "Fast-forward", Values: plan.FastForward})
	}
	if len(plan.Remember) > 0 {
		rows = append(rows, output.PlanRow{Label: "Remember", Values: plan.Remember})
	}
	return rows
}

func plannedBranches(branches []create.BranchPlan) []string {
	values := make([]string, 0, len(branches))
	for _, planned := range branches {
		switch planned.Action {
		case create.ActionCheckOut:
			values = append(values, planned.Branch+" (existing)")
		case create.ActionResume:
			values = append(values, planned.Branch+" (resumes "+planned.Upstream+")")
		case create.ActionUnchanged:
			values = append(values, planned.Branch+" (already open)")
		case create.ActionCreate:
			values = append(values, planned.Branch)
		}
	}
	return values
}

func sourceLabel(branches []create.BranchPlan) string {
	for _, planned := range branches {
		if planned.Action == create.ActionCreate {
			return "From"
		}
	}
	return "Parent"
}

func itemNote(item create.Item) string {
	note := output.SettlementNote(item.Detail.EnvPorts)
	if item.Reason == domain.ReasonUnchanged {
		note = "already open"
	}
	if note == "" {
		return ""
	}
	return output.Muted.Render(output.Dot + note)
}

func anomaliesOf(item create.Item) []string {
	var anomalies []string
	standing := item.Detail.Standing
	switch standing.OriginState {
	case worktree.Behind:
		anomalies = append(anomalies, fmt.Sprintf("%s is %d commit(s) behind origin — the worktree starts from your local branch.", item.Subject, standing.OriginBehind))
	case worktree.Diverged:
		anomalies = append(anomalies, fmt.Sprintf("%s has diverged from origin (%d ahead, %d behind) — the worktree starts from your local branch, missing commits that are on origin.", item.Subject, standing.OriginAhead, standing.OriginBehind))
	case worktree.NoOrigin, worktree.UpToDate, worktree.Ahead:
	}
	if lines := output.AnomalyLines(item.Detail.EnvPorts); len(lines) > 0 {
		anomalies = append(anomalies, anomaliesTitle+"\n"+strings.Join(lines, "\n"))
	}
	return anomalies
}

type shown struct {
	Cwd  string
	Path string
}

func shownPath(s shown) string {
	relative, err := filepath.Rel(s.Cwd, s.Path)
	if err != nil || s.Cwd == "" {
		return s.Path
	}
	return relative
}

var reasonLabels = map[domain.Reason]string{
	domain.ReasonUnchanged:   "unchanged",
	domain.ReasonIncomplete:  "left incomplete",
	domain.ReasonInterrupted: "interrupted",
	domain.ReasonNotReached:  "not reached",
}

type outcomeParams struct {
	Outcome   create.Outcome
	FollowUps []output.NextStep
	Cwd       string
}

func drawOutcome(w io.Writer, params outcomeParams) {
	items := params.Outcome.Result.Items
	headline(w, items)
	var rows []output.PlanRow
	var anomalies []string
	var next []output.NextStep
	if len(items) > 0 {
		rows = planRowsOf(params.Outcome.Plan)
	}
	for _, item := range items {
		if !stands(item) || item.Detail == nil {
			continue
		}
		path := shownPath(shown{Cwd: params.Cwd, Path: item.Detail.Path})
		rows = append(rows, output.PlanRow{Label: item.Subject, Values: []string{path + itemNote(item)}})
		anomalies = append(anomalies, anomaliesOf(item)...)
		next = append(next, output.NextStep{Command: "wtm go " + item.Subject})
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
		output.Blank(w)
		output.DrawPlan(w, output.PlanParams{Rows: rows, Warnings: anomalies})
	}
	if len(rows) == 0 && len(anomalies) > 0 {
		output.Blank(w)
		for _, text := range anomalies {
			output.WarningLine(w, text)
		}
	}
	unfinished := slices.DeleteFunc(slices.Clone(items), func(item create.Item) bool { return item.Status == domain.StatusDone })
	if len(unfinished) > 0 {
		output.Blank(w)
	}
	for _, item := range unfinished {
		unfinishedLine(w, item)
	}
	next = append(next, params.FollowUps...)
	if len(next) > 0 {
		output.Blank(w)
		output.NextSteps(w, next)
	}
}

func stands(item create.Item) bool {
	return item.Status == domain.StatusDone || item.Reason == domain.ReasonIncomplete
}

func headline(w io.Writer, items []create.Item) {
	if len(items) == 0 {
		output.UnchangedLine(w, commandName+": "+output.NothingToDo)
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
	line := commandName + ": " + strings.Join(parts, output.Dot)
	switch {
	case count["failed"] > 0:
		output.ErrorLine(w, line)
	case count["skipped"]+count["cancelled"] > 0:
		output.WarningLine(w, line)
	case count["done"] == 0:
		output.UnchangedLine(w, line)
	default:
		output.SuccessLine(w, line)
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
		output.ErrorLine(w, line)
		return
	}
	output.WarningLine(w, line)
}

func itemWhy(item create.Item) string {
	if item.Err == nil {
		return reasonLabels[item.Reason]
	}
	why := output.ErrorText(item.Err)
	if item.Reason != domain.ReasonIncomplete {
		return why
	}
	return why + output.Dot + reasonLabels[item.Reason]
}

type dryRunParams struct {
	Outcome create.Outcome
	Apply   string
}

func drawDryRun(w io.Writer, params dryRunParams) {
	output.SectionTitle(w, commandName+output.Dot+"dry run, nothing applied")
	warnings := output.Messages(params.Outcome.Warnings)
	output.DrawPlan(w, output.PlanParams{Rows: planRowsOf(params.Outcome.Plan), Warnings: warnings})
	if len(warnings) > 0 {
		output.Blank(w)
	}
	output.InfoLine(w, output.Info{Label: "fingerprint", Value: params.Outcome.Fingerprint})
	output.NextSteps(w, []output.NextStep{{Command: params.Apply, Note: applyNote}})
}

type planTextParams struct {
	Plan     create.Plan
	Warnings []domain.Warning
	Width    int
}

func planTextOf(params planTextParams) string {
	return strings.Join(output.PlanLines(output.PlanParams{Rows: planRowsOf(params.Plan), Warnings: output.Messages(params.Warnings), Width: params.Width}), "\n")
}

type envelopeParams struct {
	Outcome create.Outcome
	Err     *domain.Error
}

func envelopeOf(params envelopeParams) output.EnvelopeParams {
	outcome := params.Outcome
	doc := output.EnvelopeParams{Command: commandName, Op: outcome.Op, Err: params.Err, Flags: createFlags}
	switch {
	case outcome.Result != nil:
		doc.Err = cmp.Or(doc.Err, outcome.Failure())
		for _, item := range outcome.Result.Items {
			doc.Items = append(doc.Items, envelopeItem(item))
		}
		doc.Warnings = outcome.Warnings
	case doc.Err == nil:
		doc.Plan, doc.Fingerprint, doc.Warnings = &outcome.Plan, outcome.Fingerprint, outcome.Warnings
	}
	return doc
}

func envelopeItem(item create.Item) output.Item {
	doc := output.Item{Subject: item.Subject, Status: item.Status, Reason: item.Reason, Err: item.Err, Warnings: item.Warnings}
	if item.Detail != nil {
		doc.Detail = item.Detail
	}
	return doc
}
