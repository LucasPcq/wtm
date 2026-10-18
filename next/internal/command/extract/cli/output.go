package cli

import (
	"io"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

const commandName = extract.Name

// planRowsOf are the rows of the plan, shared by the dry run, the recap and the outcome; a value whose answer is remembered carries note.
func planRowsOf(plan extract.Plan, note string) []output.PlanRow {
	rows := []output.PlanRow{
		{Label: "Source", Values: []string{plan.Source}},
		{Label: "Files", Values: labels(plan.Files)},
		{Label: "Target", Values: []string{plannedTarget(plan)}},
	}

	if plan.Target.Action == extract.ActionCheckOut {
		rows = append(rows, output.PlanRow{Label: "Parent", Values: []string{plan.From}})
	}
	if plan.EnvFrom != "" {
		rows = append(rows, output.PlanRow{Label: "Env", Values: []string{string(plan.EnvFrom) + output.RememberedNote(plan.Remember, "env_from", note)}})
	}
	if plan.Isolation != "" {
		rows = append(rows, output.PlanRow{Label: "Isolation", Values: []string{string(plan.Isolation) + output.RememberedNote(plan.Remember, "isolation", note)}})
	}
	if ff := output.Noted(output.NotedParams{Values: plan.FastForward, Note: output.RememberedNote(plan.Remember, "fast_forward", note), None: "keep as-is"}); len(ff) > 0 {
		rows = append(rows, output.PlanRow{Label: "Fast-forward", Values: ff})
	}

	rows = append(rows, output.PlanRow{Label: "Mode", Values: []string{mode(plan.Keep)}})
	if len(plan.Conflicts) > 0 {
		rows = append(rows,
			output.PlanRow{Label: "Conflicts", Values: conflicts(plan.Conflicts)},
			output.PlanRow{Label: "On conflict", Values: []string{string(plan.OnConflict)}},
		)
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

// labels spell each file with its tag.
func labels(files []worktree.Change) []string {
	values := make([]string, 0, len(files))
	for _, file := range files {
		values = append(values, file.Status.Tag()+" "+file.Label())
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

func drawNothing(w io.Writer, plan extract.Plan) {
	output.Tally(w, output.TallyParams{Command: commandName, Note: "no uncommitted changes in " + plan.Source})
}

// drawOutcome draws what the apply did: the tally, the plan with the target when it stands, the anomalies and the conflicts left to resolve, the target left unfinished, and the next steps.
func drawOutcome(w io.Writer, outcome extract.Outcome, cwd string) {
	item, ok := targetOf(outcome)
	if !ok {
		return
	}
	detail := item.Detail
	conflicted := detail != nil && len(detail.Conflicts) > 0

	tally := output.TallyParams{Command: commandName, Items: []output.Tallied{{Status: item.Status, Reason: item.Reason}}}
	if conflicted {
		tally.Note = "with conflicts"
	}
	output.Tally(w, tally)

	rows := planRowsOf(outcome.Plan, output.Remembered)
	anomalies := anomaliesOf(outcome)
	var next []output.NextStep
	if stands(item) && detail != nil {
		path := output.ShownPath(output.Shown{Cwd: cwd, Path: detail.Path})
		rows = append(rows, output.PlanRow{Label: item.Subject, Values: []string{path + itemNote(detail, output.StylesOf(w))}})
		next = append(next, output.NextStep{Command: "wtm go " + item.Subject})
	}
	if conflicted {
		anomalies = append(anomalies, conflictNotes(conflictsParams{Detail: detail, Target: item.Subject, Source: outcome.Plan.Source})...)
		next[0].Note = "resolve the conflicts, then discard the same files in " + outcome.Plan.Source
	}

	output.Blank(w)
	output.DrawPlan(w, output.PlanParams{Rows: rows, Warnings: anomalies})

	if item.Status != domain.StatusDone {
		output.Blank(w)
		output.UnfinishedLine(w, output.Unfinished{Subject: item.Subject, Status: item.Status, Reason: item.Reason, Err: item.Err})
	}

	next = append(next, output.FollowUpSteps(item.Err)...)
	if len(next) > 0 {
		output.Blank(w)
		output.NextSteps(w, next)
	}
}

// stands reports whether the target's worktree stands: extracted to, or left incomplete by a later step of its provisioning.
func stands(item extract.Item) bool {
	return item.Status == domain.StatusDone || item.Reason == domain.ReasonIncomplete
}

// itemNote is the muted note after the target's path: how its env ports were settled, if worth saying.
func itemNote(detail *extract.Detail, st *output.Styles) string {
	note := output.SettlementNote(detail.EnvPorts)
	if note == "" {
		return ""
	}

	return st.Muted.Render(output.Dot + note)
}

// conflictsParams are the detail of a target left with conflicts, and the branches of the target and the source.
type conflictsParams struct {
	Detail *extract.Detail
	Target string
	Source string
}

// conflictNotes say which files the target holds with conflict markers, and how to undo the extract: the source still holds every change.
func conflictNotes(params conflictsParams) []string {
	conflicts := strings.Join(params.Detail.Conflicts, ", ") + " left with conflict markers in " + params.Target
	if len(params.Detail.Files) > len(params.Detail.Conflicts) {
		conflicts += "; the other files were applied cleanly"
	}

	return []string{
		conflicts + ".",
		"Nothing was removed from " + params.Source + ": to undo, discard the applied files in " + params.Target + ".",
	}
}

// anomaliesOf are what the reader should act on: the warnings of the run and of the target, and the env ports left alone.
func anomaliesOf(outcome extract.Outcome) []string {
	item, _ := targetOf(outcome)
	anomalies := append(output.Messages(outcome.Warnings), output.Messages(item.Warnings)...)

	if item.Detail == nil {
		return anomalies
	}

	return append(anomalies, output.PortAnomalies(item.Detail.EnvPorts)...)
}

func fileCount(count int) string {
	if count == 1 {
		return "1 file"
	}

	return strconv.Itoa(count) + " files"
}

// targetOf is the one item of an applied outcome, the target's; false when the outcome holds none.
func targetOf(outcome extract.Outcome) (extract.Item, bool) {
	if outcome.Result == nil || len(outcome.Result.Items) == 0 {
		return extract.Item{}, false
	}

	return outcome.Result.Items[0], true
}
