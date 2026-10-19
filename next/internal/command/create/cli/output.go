package cli

import (
	"io"
	"slices"

	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

// preview is what the recap and the dry run show of the plan previewed.
func preview(previewed create.Outcome) form.Preview {
	plan := previewed.Plan
	return form.Preview{Rows: output.RecapRows(plan.Recap), Warnings: previewed.Warnings, Recalled: plan.Recalled}
}

// itemNote is the muted note after an item's path: already open, else how its env ports were settled, if worth saying.
func itemNote(item create.Item, st *output.Styles) string {
	if item.Reason == domain.ReasonUnchanged {
		return output.MutedNote("already open", st)
	}

	return output.MutedNote(output.SettlementNote(item.Detail.EnvPorts), st)
}

// drawOutcome draws what the apply did: the tally, the plan with each worktree that exists and the warnings, each item's under its subject, the branches left unfinished, and the next steps, among them the commands the failed branches suggest to repair what they left behind.
func drawOutcome(w io.Writer, outcome create.Outcome, env uicli.Env) {
	items := outcome.Result.Items
	headline(w, items)

	var rows []output.PlanRow
	var anomalies []string
	var next []output.NextStep
	if len(items) > 0 {
		rows = output.RecapRows(outcome.Plan.Recap)
	}

	for _, item := range items {
		if item.Detail == nil {
			continue
		}
		path := output.ShownPath(output.Shown{Cwd: env.Cwd, Path: item.Detail.Path})
		rows = append(rows, output.PlanRow{Label: item.Subject, Values: []string{path + itemNote(item, output.StylesOf(w))}})
		next = append(next, output.GoStep(item.Subject))
	}

	for _, item := range items {
		for _, message := range output.WarningMessages(item.Warnings, envPortsOf(item)) {
			anomalies = append(anomalies, item.Subject+"  "+message)
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
		next = append(next, output.FollowUpSteps(item.Err, env.CommandLine)...)
	}
	if len(next) > 0 {
		output.Blank(w)
		output.NextSteps(w, next)
	}
}

func envPortsOf(item create.Item) env.Redacted {
	if item.Detail == nil {
		return env.Redacted{}
	}

	return item.Detail.EnvPorts
}

// headline is the tally of the items by status.
func headline(w io.Writer, items []create.Item) {
	tallied := make([]output.Tallied, 0, len(items))
	for _, item := range items {
		tallied = append(tallied, output.Tallied{Status: item.Status, Reason: item.Reason})
	}

	output.Tally(w, output.TallyParams{Command: create.Name, Items: tallied})
}
