package cli

import (
	"io"

	"github.com/LucasPcq/wtm/next/internal/command/daemonstop"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

// drawOutcome draws the conclusion of the stop: the tally, the plan, the warnings and the follow-ups; with no daemon running, that none runs.
func drawOutcome(w io.Writer, outcome daemonstop.Outcome, view output.View) {
	note := ""
	if !outcome.Plan.Running {
		note = "wtm's daemon is not running"
	}
	output.DrawApplied(w, outcome, output.AppliedParams{Command: Use, Note: note, View: view})
}
