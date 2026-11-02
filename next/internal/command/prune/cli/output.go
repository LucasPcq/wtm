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
	output.DrawApplied(w, outcome, output.AppliedParams{Command: prune.Name, View: view, Noun: owedNoun(outcome.Result.Items), Block: func(w io.Writer) { drawPaid(w, outcome.Result.Items) }})
}

// owedNoun counts the drops owed the run paid by their noun, apart from the worktrees pruned.
func owedNoun(items []prune.Item) func(subject string) string {
	return func(subject string) string {
		at := slices.IndexFunc(items, func(item prune.Item) bool { return item.Subject == subject })
		if at < 0 || items[at].Detail == nil || items[at].Detail.Owed == nil {
			return ""
		}
		return owedDrop
	}
}

// owedDrop is how the tally counts a drop owed.
const owedDrop = "owed drop"

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
