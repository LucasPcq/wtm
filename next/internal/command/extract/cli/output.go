package cli

import (
	"io"

	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

func drawNothing(w io.Writer, plan extract.Plan) {
	output.Tally(w, output.TallyParams{Command: extract.Name, Note: plan.Summary.Message})
}

func drawOutcome(w io.Writer, outcome extract.Outcome, view output.View) {
	output.DrawApplied(w, outcome, output.AppliedParams{Command: extract.Name, Note: outcome.Plan.Summary.Message, View: view})
}
