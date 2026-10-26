package cli

import (
	"io"

	"github.com/LucasPcq/wtm/next/internal/command/clean"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

func drawOutcome(w io.Writer, outcome clean.Outcome, view output.View) {
	output.DrawApplied(w, outcome, output.AppliedParams{Command: clean.Name, View: view})
}
