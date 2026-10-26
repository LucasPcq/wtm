package cli

import (
	"io"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/command/exec"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

// drawOutcome draws, with --print, each target's whole output under its name, then the conclusion of the run.
func drawOutcome(w io.Writer, outcome exec.Outcome, view output.View) {
	if outcome.Plan.Print {
		for _, item := range outcome.Result.Items {
			if item.Detail == nil {
				continue
			}
			output.SectionTitle(w, item.Subject)
			for line := range strings.Lines(item.Detail.Output) {
				output.Message(w, output.Indent+strings.TrimSuffix(line, "\n"))
			}
			output.Blank(w)
		}
	}

	output.DrawApplied(w, outcome, output.AppliedParams{Command: exec.Name, View: view})
}
