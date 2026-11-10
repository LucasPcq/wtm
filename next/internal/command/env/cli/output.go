package cli

import (
	"io"

	envcmd "github.com/LucasPcq/wtm/next/internal/command/env"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

func drawOutcome(w io.Writer, outcome envcmd.Outcome, view output.View) {
	output.DrawApplied(w, outcome, output.AppliedParams{Command: envcmd.Name, View: view, Unchanged: "in step"})
}
