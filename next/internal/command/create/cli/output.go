package cli

import (
	"io"

	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

func drawOutcome(w io.Writer, outcome create.Outcome, view output.View) {
	output.DrawApplied(w, outcome, output.AppliedParams{Command: create.Name, Unchanged: "already open", View: view})
}
