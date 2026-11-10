package cli

import (
	"io"

	"github.com/LucasPcq/wtm/next/internal/command/initialize"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

func drawOutcome(w io.Writer, outcome initialize.Outcome, view output.View) {
	output.DrawApplied(w, outcome, output.AppliedParams{Command: initialize.Name, View: view, Unchanged: "nothing to change"})
}
