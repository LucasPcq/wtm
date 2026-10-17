package cli

import (
	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

// An extractLine is the command line that runs a request again: the apply line after a dry run, the preview line after a stale plan.
type extractLine struct {
	Command string
	Request extract.Request
}

// Argv spells the command and its request word by word, as extract's form spells its flags.
func (l extractLine) Argv() []string {
	return newForm(nil).Argv(l.Command, l.Request)
}

// String spells the command line for the shell.
func (l extractLine) String() string {
	return output.Line(l.Argv())
}
