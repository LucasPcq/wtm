package cli

import (
	"io"

	"github.com/LucasPcq/wtm/next/internal/command/fastforward"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

func drawOutcome(w io.Writer, outcome fastforward.Outcome, view output.View) {
	output.DrawApplied(w, outcome, output.AppliedParams{Command: fastforward.Name, View: view})
}
