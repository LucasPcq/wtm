package cli

import (
	"io"

	"github.com/LucasPcq/wtm/next/internal/command/rundown"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

// drawOutcome draws the conclusion of the stop: the tally, the plan, the warnings and the follow-ups.
func drawOutcome(w io.Writer, outcome rundown.Outcome, view output.View) {
	output.DrawApplied(w, outcome, output.AppliedParams{Command: Use, View: view})
}
