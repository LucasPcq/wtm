package cli

import (
	"io"

	"github.com/LucasPcq/wtm/next/internal/command/runinit"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

func drawOutcome(w io.Writer, outcome runinit.Outcome, view output.View) {
	output.DrawApplied(w, outcome, output.AppliedParams{Command: runinit.Name, View: view, Unchanged: "nothing to change"})
}
