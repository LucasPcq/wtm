package cli

import (
	"io"

	"github.com/LucasPcq/wtm/next/internal/command/daemonstopjobs"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

// drawOutcome draws the conclusion of the stop: the tally, the plan, the warnings, each worktree whose jobs did not all stop and the follow-ups.
func drawOutcome(w io.Writer, outcome daemonstopjobs.Outcome, view output.View) {
	output.DrawApplied(w, outcome, output.AppliedParams{Command: Use, View: view})
}
