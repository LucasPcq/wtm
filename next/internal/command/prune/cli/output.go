package cli

import (
	"io"
	"slices"

	"github.com/LucasPcq/wtm/next/internal/command/prune"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

// drawOutcome draws the conclusion of the prune: the tally, the plan but the drops owed it planned, then each drop owed made, the others among the items not done.
func drawOutcome(w io.Writer, outcome prune.Outcome, view output.View) {
	outcome.Plan.Recap = slices.DeleteFunc(slices.Clone(outcome.Plan.Recap), func(row domain.RecapRow) bool { return row.Key == prune.RecapOwed })
	output.DrawApplied(w, outcome, output.AppliedParams{Command: prune.Name, View: view, Block: func(w io.Writer) { drawPaid(w, outcome.Result.Items) }})
}

// drawPaid draws a line for each drop owed the run made.
func drawPaid(w io.Writer, items []prune.Item) {
	first := true
	for _, item := range items {
		if item.Detail == nil || item.Detail.Owed == nil || item.Status != domain.StatusDone {
			continue
		}
		if first {
			output.Blank(w)
			first = false
		}
		debt := item.Detail.Owed
		output.SuccessLine(w, "dropped "+debt.Name+" from "+debt.Job+", owed since "+debt.Worktree+" was removed")
	}
}
