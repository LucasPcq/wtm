package cli

import (
	"strings"

	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

// A createLine is the command line that runs a request again: the apply line after a dry run, the preview line after a stale plan.
type createLine struct {
	Command string
	Request create.Request
}

// String spells the command and its request, the flags in a fixed order and each only when given.
func (l createLine) String() string {
	req := l.Request
	words := append([]string{l.Command}, output.Quoted(req.Branches)...)
	words = append(words, output.FlagWords(output.FlagValue{Name: "env-from", Value: string(req.EnvFrom)})...)

	switch {
	case req.FastForward == nil:
	case *req.FastForward:
		words = append(words, "--ff")
	default:
		words = append(words, "--ff=false")
	}

	words = append(words, output.FlagWords(output.FlagValue{Name: "from", Value: req.From})...)
	if req.IfNotExists {
		words = append(words, "--if-not-exists")
	}

	words = append(words, output.FlagWords(output.FlagValue{Name: "isolation", Value: string(req.Isolation)})...)

	for _, answer := range req.Remember {
		words = append(words, output.FlagWords(output.FlagValue{Name: "remember", Value: answer})...)
	}

	if req.Ask {
		words = append(words, "--ask")
	}

	return strings.Join(words, " ")
}
