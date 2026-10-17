package cli

import (
	"strings"

	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

// An extractLine is the command line that runs a request again: the apply line after a dry run, the preview line after a stale plan.
type extractLine struct {
	Command string
	Request extract.Request
}

// String is the command line of Argv, quoted for the shell.
func (l extractLine) String() string {
	return output.Line(l.Argv())
}

// Argv spells the command and its request word by word, the flags each only when given.
func (l extractLine) Argv() []string {
	req := l.Request
	words := strings.Fields(l.Command)
	if req.Source != "" {
		words = append(words, req.Source)
	}

	for _, file := range req.Files {
		words = append(words, output.FlagWords(output.FlagValue{Name: "files", Value: file})...)
	}

	words = append(words, output.FlagWords(output.FlagValue{Name: "to", Value: req.To})...)
	words = append(words, output.SwitchWords(flagFF, req.FastForward)...)
	words = append(words, output.FlagWords(output.FlagValue{Name: "from", Value: req.From})...)
	words = append(words, output.FlagWords(output.FlagValue{Name: "isolation", Value: string(req.Isolation)})...)
	words = append(words, output.SwitchWords(flagKeep, req.Keep)...)
	words = append(words, output.FlagWords(output.FlagValue{Name: "on-conflict", Value: string(req.OnConflict)})...)

	for _, answer := range req.Remember {
		words = append(words, output.FlagWords(output.FlagValue{Name: "remember", Value: answer})...)
	}

	if req.Ask {
		words = append(words, "--ask")
	}

	return words
}
