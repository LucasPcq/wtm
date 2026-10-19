package cli

import (
	"io"
	"slices"

	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

// preview is what the recap and the dry run show of the plan previewed.
func preview(previewed extract.Outcome) form.Preview {
	plan := previewed.Plan
	return form.Preview{Rows: output.RecapRows(plan.Recap), Warnings: previewed.Warnings, Recalled: plan.Recalled}
}

func drawNothing(w io.Writer, plan extract.Plan) {
	output.Tally(w, output.TallyParams{Command: extract.Name, Note: "no uncommitted changes in " + plan.Source})
}

// drawOutcome draws what the apply did: the tally, the plan with the target when it stands and the warnings, the target left unfinished followed by its own warnings, and the next steps.
func drawOutcome(w io.Writer, outcome extract.Outcome, cwd string) {
	item, ok := targetOf(outcome)
	if !ok {
		return
	}
	detail := item.Detail
	conflicted := detail != nil && len(detail.Conflicts) > 0
	finished := item.Status == domain.StatusDone

	tally := output.TallyParams{Command: extract.Name, Items: []output.Tallied{{Status: item.Status, Reason: item.Reason}}}
	if conflicted {
		tally.Note = "with conflicts"
	}
	output.Tally(w, tally)

	rows := output.RecapRows(outcome.Plan.Recap)
	own, note := stepOf(item.Warnings)
	targetWarnings := output.WarningMessages(own, envPortsOf(item))
	warnings := output.Messages(outcome.Warnings)
	if finished {
		warnings = append(warnings, targetWarnings...)
	}
	var next []output.NextStep
	if detail != nil {
		path := output.ShownPath(output.Shown{Cwd: cwd, Path: detail.Path})
		rows = append(rows, output.PlanRow{Label: item.Subject, Values: []string{path + output.MutedNote(output.SettlementNote(detail.EnvPorts), output.StylesOf(w))}})
		step := output.GoStep(item.Subject)
		step.Note = note
		next = append(next, step)
	}

	output.Blank(w)
	output.DrawPlan(w, output.PlanParams{Rows: rows, Warnings: warnings})

	if !finished {
		output.Blank(w)
		output.UnfinishedLine(w, output.Unfinished{Subject: item.Subject, Status: item.Status, Reason: item.Reason, Err: item.Err})
		for _, text := range targetWarnings {
			output.WarningLine(w, text)
		}
	}

	next = append(next, output.FollowUpSteps(item.Err)...)
	if len(next) > 0 {
		output.Blank(w)
		output.NextSteps(w, next)
	}
}

// stepOf parts the target's warnings from the step left to take, which the step to the target carries as its note.
func stepOf(warnings []domain.Warning) ([]domain.Warning, string) {
	at := slices.IndexFunc(warnings, func(warned domain.Warning) bool { return warned.Code == extract.CodeResolveThenDiscard })
	if at < 0 {
		return warnings, ""
	}

	return slices.Delete(slices.Clone(warnings), at, at+1), warnings[at].Message
}

func envPortsOf(item extract.Item) env.Redacted {
	if item.Detail == nil {
		return env.Redacted{}
	}

	return item.Detail.EnvPorts
}

// targetOf is the one item of an applied outcome, the target's; false when the outcome holds none.
func targetOf(outcome extract.Outcome) (extract.Item, bool) {
	if outcome.Result == nil || len(outcome.Result.Items) == 0 {
		return extract.Item{}, false
	}

	return outcome.Result.Items[0], true
}
