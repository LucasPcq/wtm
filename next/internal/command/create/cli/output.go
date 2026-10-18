package cli

import (
	"fmt"
	"io"
	"slices"

	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

// planRowsOf are the rows of the plan, shared by the dry run, the recap and the outcome, each answer with its mark.
func planRowsOf(plan create.Plan, marks output.Marks) []output.PlanRow {
	rows := []output.PlanRow{
		{Label: "Branches", Values: plannedBranches(plan.Branches)},
		{Label: sourceLabel(plan.Branches), Values: []string{plan.From}},
		{Label: "Env", Values: []string{output.EnvValue(output.EnvValueParams{Strategy: string(plan.EnvFrom), FollowsConfig: plan.EnvFollowsConfig(), Mark: marks.Of(domain.PathEnvFrom)})}},
	}

	if plan.Isolation != "" {
		rows = append(rows, output.PlanRow{Label: "Isolation", Values: []string{string(plan.Isolation) + marks.Of(domain.PathIsolation)}})
	}
	if ff := output.Noted(output.NotedParams{Values: plan.FastForward, Note: marks.Of(domain.PathFastForward), None: "keep as-is"}); len(ff) > 0 {
		rows = append(rows, output.PlanRow{Label: "Fast-forward", Values: ff})
	}

	return rows
}

// preview is what the recap and the dry run show of the plan previewed.
func preview(previewed create.Outcome) form.Preview {
	plan := previewed.Plan
	return form.Preview{Rows: planRowsOf(plan, previewMarks(plan)), Warnings: previewed.Warnings, Recalled: plan.Recalled()}
}

// previewMarks are the marks of a plan not yet applied.
func previewMarks(plan create.Plan) output.Marks {
	return output.Marks{Recalled: plan.Recalled(), Remembered: plan.Remember, Mark: output.WillBeRemembered}
}

// plannedBranches spells each planned branch, how it gets its worktree in parentheses unless it is created.
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

// sourceLabel names the source row: From when a branch is created from it, else Parent, the branch recorded for wtm sync.
func sourceLabel(branches []create.BranchPlan) string {
	for _, planned := range branches {
		if planned.Action == create.ActionCreate {
			return "From"
		}
	}

	return "Parent"
}

// itemNote is the muted note after an item's path: already open, else how its env ports were settled, if worth saying.
func itemNote(item create.Item, st *output.Styles) string {
	if item.Reason == domain.ReasonUnchanged {
		return output.MutedNote("already open", st)
	}

	return output.MutedNote(output.SettlementNote(item.Detail.EnvPorts), st)
}

// anomaliesOf are what the reader of an item should act on: a branch checked out behind origin, the env ports left alone.
func anomaliesOf(item create.Item) []string {
	var anomalies []string
	standing := item.Detail.Standing
	switch standing.OriginState {
	case worktree.Behind:
		anomalies = append(anomalies, fmt.Sprintf("%s is %d commit(s) behind origin — the worktree starts from your local branch.", item.Subject, standing.OriginBehind))
	// A diverged branch is the plan's create.source_diverged warning already.
	case worktree.Diverged, worktree.NoOrigin, worktree.UpToDate, worktree.Ahead:
	}

	return append(anomalies, output.PortAnomalies(item.Detail.EnvPorts)...)
}

// drawOutcome draws what the apply did: the tally, the plan with each worktree that exists and the anomalies, the branches left unfinished, and the next steps, among them the commands the failed branches suggest to repair what they left behind.
func drawOutcome(w io.Writer, outcome create.Outcome, cwd string) {
	items := outcome.Result.Items
	headline(w, items)

	var rows []output.PlanRow
	var anomalies []string
	var next []output.NextStep
	if len(items) > 0 {
		rows = planRowsOf(outcome.Plan, output.Marks{Recalled: outcome.Plan.Recalled(), Remembered: outcome.Result.Remembered, Mark: output.Saved})
	}

	for _, item := range items {
		if item.Detail == nil {
			continue
		}
		path := output.ShownPath(output.Shown{Cwd: cwd, Path: item.Detail.Path})
		rows = append(rows, output.PlanRow{Label: item.Subject, Values: []string{path + itemNote(item, output.StylesOf(w))}})
		anomalies = append(anomalies, anomaliesOf(item)...)
		next = append(next, output.GoStep(item.Subject))
	}

	for _, item := range items {
		for _, warned := range item.Warnings {
			anomalies = append(anomalies, item.Subject+"  "+warned.Message)
		}
	}

	for _, warned := range outcome.Warnings {
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
		output.UnfinishedLine(w, output.Unfinished{Subject: item.Subject, Status: item.Status, Reason: item.Reason, Err: item.Err})
	}

	for _, item := range items {
		next = append(next, output.FollowUpSteps(item.Err)...)
	}
	if len(next) > 0 {
		output.Blank(w)
		output.NextSteps(w, next)
	}
}

// headline is the tally of the items by status.
func headline(w io.Writer, items []create.Item) {
	tallied := make([]output.Tallied, 0, len(items))
	for _, item := range items {
		tallied = append(tallied, output.Tallied{Status: item.Status, Reason: item.Reason})
	}

	output.Tally(w, output.TallyParams{Command: create.Name, Items: tallied})
}
