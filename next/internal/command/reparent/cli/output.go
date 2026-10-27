package cli

import (
	"io"

	"github.com/LucasPcq/wtm/next/internal/command/reparent"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

func drawOutcome(w io.Writer, outcome reparent.Outcome, view output.View) {
	output.DrawApplied(w, outcome, output.AppliedParams{Command: reparent.Name, View: view})
}
