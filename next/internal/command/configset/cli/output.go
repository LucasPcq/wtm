package cli

import (
	"io"

	"github.com/LucasPcq/wtm/next/internal/command/configset"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

// drawOutcome draws the conclusion of the change: the tally, what config.toml now holds, the warnings and the follow-up that puts it back.
func drawOutcome(w io.Writer, outcome configset.Outcome, view output.View) {
	note := ""
	if outcome.Plan.Empty {
		note = "config.toml already holds it"
	}
	output.DrawApplied(w, outcome, output.AppliedParams{Command: Use, Note: note, View: view})
}
