package cli

import (
	"io"

	"github.com/LucasPcq/wtm/next/internal/command/configedit"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

// drawEdit draws what the editor left: that config.toml is one wtm can use, else each problem and the edit again.
func drawEdit(w io.Writer, read configedit.Outcome, _ configedit.Request, view output.View) {
	if read.Failure() == nil {
		output.SuccessLine(w, "config.toml: every setting is one wtm can use")
	}
	output.ReadEnd(w, read)
	if failure := read.Failure(); failure != nil && failure.FollowUp != nil {
		output.Blank(w)
		output.NextSteps(w, []output.NextStep{output.FollowUpStep(*failure.FollowUp, view.CommandLine)})
	}
}
