package cli

import (
	"io"

	"github.com/LucasPcq/wtm/next/internal/command/sync"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

func drawOutcome(w io.Writer, outcome sync.Outcome, view output.View) {
	output.DrawApplied(w, outcome, output.AppliedParams{Command: sync.Name, View: view})
}
