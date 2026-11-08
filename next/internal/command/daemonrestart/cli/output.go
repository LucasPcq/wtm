package cli

import (
	"io"

	"github.com/LucasPcq/wtm/next/internal/command/daemonrestart"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

// drawOutcome draws the conclusion of the restart: the tally, the plan (what came back, what was taken over, what was not), the warnings, each job that did not come back and the follow-ups.
func drawOutcome(w io.Writer, outcome daemonrestart.Outcome, view output.View) {
	output.DrawApplied(w, outcome, output.AppliedParams{Command: Use, View: view})
}
