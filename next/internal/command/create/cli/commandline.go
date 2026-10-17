package cli

import (
	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

// A createLine is the command line that runs a request again: the apply line after a dry run, the preview line after a stale plan.
type createLine struct {
	Command string
	Request create.Request
}

// Argv spells the command and its request word by word, as create's form spells its flags.
func (l createLine) Argv() []string {
	return newForm(nil).Argv(l.Command, l.Request)
}

// String spells the command line for the shell.
func (l createLine) String() string {
	return output.Line(l.Argv())
}
