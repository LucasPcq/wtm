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

	output.DrawApplied(w, outcome, output.AppliedParams{Command: exec.Name, View: view, Excerpt: excerpt(outcome)})
}

// excerpt is the last lines and the log of each target of outcome, by its name.
func excerpt(outcome exec.Outcome) func(subject string) ([]string, string) {
	return func(subject string) ([]string, string) {
		for _, item := range outcome.Result.Items {
			if item.Subject == subject && item.Detail != nil {
				return item.Detail.Tail, item.Detail.Log
			}
		}
		return nil, ""
	}
}
