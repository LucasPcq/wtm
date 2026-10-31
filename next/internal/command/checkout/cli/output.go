package cli

import (
	"io"

	"github.com/LucasPcq/wtm/next/internal/command/checkout"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

func drawOutcome(w io.Writer, outcome checkout.Outcome, view output.View) {
	output.DrawApplied(w, outcome, output.AppliedParams{Command: checkout.Name, View: view})
}
