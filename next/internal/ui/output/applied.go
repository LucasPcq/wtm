package output

import (
	"io"
	"slices"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
)

// A RecappedPlan carries what its preview and its outcome show of it: it embeds domain.Recapped.
type RecappedPlan interface {
	Recaps() domain.Recapped
}

// A Worktree is the detail of an item whose worktree stands: where it is and the env ports it was given.
type Worktree interface {
	Worktree() (path string, ports env.Redacted)
}

// AppliedParams is what DrawApplied draws an outcome as, beyond the outcome itself: the command it counts for, the note its tally adds, and the view it draws from.
type AppliedParams struct {
	Command string
	Note    string
	View    View
}

// DrawApplied draws what the apply of a mutation did: the tally, the applied plan with a row per worktree that stands and the warnings of the items done then of the run, each item left unfinished followed by its own warnings, and the next steps, the result's then each item's: what repairs what its failure left behind, then what its warnings offer. An item's warning reads subject, then its message.
func DrawApplied[P RecappedPlan, D Worktree](w io.Writer, outcome domain.Outcome[P, D], params AppliedParams) {
	items := outcome.Result.Items
	tallied := make([]Tallied, 0, len(items))
	for _, item := range items {
		tallied = append(tallied, Tallied{Status: item.Status, Reason: item.Reason})
	}
	Tally(w, TallyParams{Command: params.Command, Items: tallied, Note: params.Note})

	var rows []PlanRow
	var warnings []string
	if len(items) > 0 {
		rows = RecapRows(outcome.Plan.Recaps().Recap)
	}
	for _, item := range items {
		if item.Detail != nil {
			rows = append(rows, worktreeRow(w, item, params.View))
		}
		if item.Status == domain.StatusDone {
			warnings = append(warnings, itemWarnings(item)...)
		}
	}
	warnings = append(warnings, Messages(outcome.Warnings)...)

	if len(rows) > 0 {
		Blank(w)
		DrawPlan(w, PlanParams{Rows: rows, Warnings: warnings})
	}
	if len(rows) == 0 && len(warnings) > 0 {
		Blank(w)
		for _, text := range warnings {
			WarningLine(w, text)
		}
	}

	drawUnfinished(w, items)

	var next []NextStep
	for _, followUp := range outcome.Result.FollowUps {
		next = append(next, FollowUpStep(followUp, params.View.CommandLine))
	}
	for _, item := range items {
		next = append(next, FollowUpSteps(item.Err, params.View.CommandLine)...)
		next = append(next, WarningSteps(item.Warnings, params.View.CommandLine)...)
	}
	if len(next) > 0 {
		Blank(w)
		NextSteps(w, next)
	}
}

// worktreeRow is the row of an item's worktree: its path, then muted, already open or how its env ports were settled, if worth saying.
func worktreeRow[D Worktree](w io.Writer, item domain.Unit[D], view View) PlanRow {
	path, ports := (*item.Detail).Worktree()
	note := SettlementNote(ports)
	if item.Reason == domain.ReasonUnchanged {
		note = "already open"
	}

	return PlanRow{Label: item.Subject, Values: []string{ShownPath(Shown{Cwd: view.Cwd, Path: path}) + MutedNote(note, StylesOf(w))}}
}

func drawUnfinished[D Worktree](w io.Writer, items []domain.Unit[D]) {
	unfinished := slices.DeleteFunc(slices.Clone(items), func(item domain.Unit[D]) bool { return item.Status == domain.StatusDone })
	if len(unfinished) > 0 {
		Blank(w)
	}
	for _, item := range unfinished {
		UnfinishedLine(w, Unfinished{Subject: item.Subject, Status: item.Status, Reason: item.Reason, Err: item.Err})
		for _, text := range itemWarnings(item) {
			WarningLine(w, text)
		}
	}
}

func itemWarnings[D Worktree](item domain.Unit[D]) []string {
	messages := WarningMessages(item.Warnings)
	for i, message := range messages {
		messages[i] = item.Subject + "  " + message
	}

	return messages
}
