package cli

import (
	"cmp"
	"io"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

const (
	commandName    = "extract"
	anomaliesTitle = "Env ports left alone"
)

// planRowsOf are the rows of the plan the dry run shows.
func planRowsOf(plan extract.Plan) []output.PlanRow {
	rows := []output.PlanRow{
		{Label: "Source", Values: []string{plan.Source}},
		{Label: "Files", Values: labels(plan.Files)},
		{Label: "Target", Values: []string{plannedTarget(plan)}},
	}

	if plan.Target.Action == extract.ActionCheckOut {
		rows = append(rows, output.PlanRow{Label: "Parent", Values: []string{plan.From}})
	}
	if plan.Isolation != "" {
		rows = append(rows, output.PlanRow{Label: "Isolation", Values: []string{string(plan.Isolation)}})
	}
	if len(plan.FastForward) > 0 {
		rows = append(rows, output.PlanRow{Label: "Fast-forward", Values: plan.FastForward})
	}

	rows = append(rows, output.PlanRow{Label: "Mode", Values: []string{mode(plan.Keep)}})
	if len(plan.Conflicts) > 0 {
		rows = append(rows,
			output.PlanRow{Label: "Conflicts", Values: conflicts(plan.Conflicts)},
			output.PlanRow{Label: "On conflict", Values: []string{string(plan.OnConflict)}},
		)
	}
	if len(plan.Remember) > 0 {
		rows = append(rows, output.PlanRow{Label: "Remember", Values: plan.Remember})
	}

	return rows
}

// plannedTarget spells the target and how it gets its worktree; an existing one is its branch alone.
func plannedTarget(plan extract.Plan) string {
	target := plan.Target
	switch target.Action {
	case extract.ActionCreate:
		return "new worktree " + target.Branch + " from " + plan.From
	case extract.ActionCheckOut:
		return target.Branch + " (existing local branch — reused)"
	case extract.ActionResume:
		return target.Branch + " (resumes " + target.Upstream + ")"
	case extract.ActionExisting:
	}

	return target.Branch
}

func labels(files []worktree.Change) []string {
	values := make([]string, 0, len(files))
	for _, file := range files {
		values = append(values, file.Label())
	}

	return values
}

func conflicts(found []worktree.Conflict) []string {
	values := make([]string, 0, len(found))
	for _, conflict := range found {
		values = append(values, conflict.Path+" ("+string(conflict.Kind)+")")
	}

	return values
}

func mode(keep bool) string {
	if keep {
		return "copy"
	}

	return "move"
}

// drawDryRun draws the plan and its fingerprint, and apply, the line that applies exactly that plan.
func drawDryRun(w io.Writer, outcome extract.Outcome, apply string) {
	output.SectionTitle(w, commandName+output.Dot+"dry run, nothing applied")
	warnings := output.Messages(outcome.Warnings)
	output.DrawPlan(w, output.PlanParams{Rows: planRowsOf(outcome.Plan), Warnings: warnings})
	if len(warnings) > 0 {
		output.Blank(w)
	}

	output.InfoLine(w, output.Info{Label: "fingerprint", Value: outcome.Fingerprint})
	output.NextSteps(w, []output.NextStep{{Command: apply, Note: output.ApplyNote}})
}

func drawNothing(w io.Writer, plan extract.Plan) {
	output.UnchangedLine(w, "No uncommitted changes to extract in "+plan.Source)
}

// drawOutcome draws what the apply did: the files moved or the conflicts left to resolve, where the source and the target stand, the anomalies, and the next step; an unfinished target is drawn by drawUnfinished.
func drawOutcome(w io.Writer, outcome extract.Outcome, cwd string) {
	item := outcome.Result.Item
	if item.Status != domain.StatusDone {
		drawUnfinished(w, outcome, cwd)
		return
	}

	detail := item.Detail
	source := outcome.Plan.Source
	conflicted := len(detail.Conflicts) > 0

	var rows []output.PlanRow
	next := output.NextStep{Command: "wtm go " + item.Subject}
	if conflicted {
		drawConflicts(w, conflictsParams{Detail: detail, Target: item.Subject, Source: source})
		next.Note = "resolve the conflicts, then discard the same files in " + source
	} else {
		drawApplied(w, detail, item.Subject)
		rows = append(rows, output.PlanRow{Label: "source", Values: []string{source + output.Dot + sourceNote(detail.Kept)}})
	}

	if note := output.SettlementNote(detail.EnvPorts); note != "" {
		rows = append(rows, output.PlanRow{Label: "env", Values: []string{note}})
	}
	rows = append(rows, output.PlanRow{Label: "path", Values: []string{output.ShownPath(output.Shown{Cwd: cwd, Path: detail.Path})}})

	output.Blank(w)
	output.DrawPlan(w, output.PlanParams{Rows: rows, Warnings: anomaliesOf(outcome)})

	output.Blank(w)
	output.NextSteps(w, []output.NextStep{next})
}

// drawApplied draws the files moved, or copied, cleanly into target.
func drawApplied(w io.Writer, detail *extract.Detail, target string) {
	verb := "Moved "
	if detail.Kept {
		verb = "Copied "
	}

	output.SuccessLine(w, verb+fileCount(len(detail.Files))+" to "+target)
	output.Blank(w)
	for _, file := range detail.Files {
		output.Message(w, output.Indent+file.Status.Tag()+"  "+file.Path)
	}
}

// conflictsParams are the detail of a target left with conflicts, and the branches of the target and the source.
type conflictsParams struct {
	Detail *extract.Detail
	Target string
	Source string
}

// drawConflicts draws the files left with conflict markers in the target, and how to undo the extract: the source still holds every change.
func drawConflicts(w io.Writer, params conflictsParams) {
	output.WarningLine(w, "Applied to "+params.Target+" with conflicts")
	output.Blank(w)
	output.SectionTitle(w, "Conflicts to resolve")
	for _, path := range params.Detail.Conflicts {
		output.Message(w, output.Indent+path)
	}

	output.Blank(w)
	if len(params.Detail.Files) > len(params.Detail.Conflicts) {
		output.Message(w, "The other files were applied cleanly.")
	}

	output.Message(w, "Nothing was removed from "+params.Source+": to undo, discard the applied files in "+params.Target+".")
}

func fileCount(count int) string {
	if count == 1 {
		return "1 file"
	}

	return strconv.Itoa(count) + " files"
}

func sourceNote(kept bool) string {
	if kept {
		return "files kept"
	}

	return "files removed"
}

// anomaliesOf are what the reader should act on: the warnings of the run and of the target, and the env ports left alone.
func anomaliesOf(outcome extract.Outcome) []string {
	item := outcome.Result.Item
	anomalies := append(output.Messages(outcome.Warnings), output.Messages(item.Warnings)...)

	if item.Detail == nil {
		return anomalies
	}

	if lines := output.AnomalyLines(item.Detail.EnvPorts); len(lines) > 0 {
		anomalies = append(anomalies, anomaliesTitle+"\n"+strings.Join(lines, "\n"))
	}

	return anomalies
}

// drawUnfinished draws a target that failed, was skipped or was cancelled: why, where its worktree stands if it does, and the command that repairs what it left behind.
func drawUnfinished(w io.Writer, outcome extract.Outcome, cwd string) {
	item := outcome.Result.Item
	output.UnfinishedLine(w, output.Unfinished{Subject: item.Subject, Status: item.Status, Reason: item.Reason, Err: item.Err})

	if item.Detail != nil {
		output.Blank(w)
		path := output.ShownPath(output.Shown{Cwd: cwd, Path: item.Detail.Path})
		output.DrawPlan(w, output.PlanParams{Rows: []output.PlanRow{{Label: "path", Values: []string{path}}}, Warnings: anomaliesOf(outcome)})
	}

	if item.Err == nil || item.Err.FollowUp == nil {
		return
	}

	output.Blank(w)
	output.NextSteps(w, []output.NextStep{{Command: output.FollowUpLine(item.Err.FollowUp), Note: item.Err.FollowUp.Message}})
}

type envelopeParams struct {
	Outcome extract.Outcome
	Err     *domain.Error
}

// envelopeOf maps how the run ended onto the v2 envelope: its item once applied, else the plan and its fingerprint unless refused.
func envelopeOf(params envelopeParams) output.EnvelopeParams {
	outcome := params.Outcome
	doc := output.EnvelopeParams{Command: commandName, Op: outcome.Op, Err: params.Err, Flags: extractFlags}

	switch {
	case outcome.Result != nil:
		item := outcome.Result.Item
		doc.Err = cmp.Or(doc.Err, outcome.Failure())
		doc.Items = []output.Item{{Subject: item.Subject, Status: item.Status, Reason: item.Reason, Err: item.Err, Warnings: item.Warnings}}
		// Set only when there is one: a nil *Detail would make the field a non-nil interface, encoded null instead of {}.
		if item.Detail != nil {
			doc.Items[0].Detail = item.Detail
		}
		doc.Warnings = outcome.Warnings
	case doc.Err == nil:
		doc.Plan, doc.Fingerprint, doc.Warnings = &outcome.Plan, outcome.Fingerprint, outcome.Warnings
	}

	return doc
}
