package cli

import (
	"io"

	"github.com/LucasPcq/wtm/next/internal/command/prune"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

func drawOutcome(w io.Writer, outcome prune.Outcome, view output.View) {
	output.DrawApplied(w, outcome, output.AppliedParams{Command: prune.Name, View: view})
}
