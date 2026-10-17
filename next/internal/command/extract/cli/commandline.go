package cli

import (
	"strings"

	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

type extractLine struct {
	Command string
	Request extract.Request
}

func (l extractLine) String() string {
	req := l.Request
	words := []string{l.Command}
	if req.Source != "" {
		words = append(words, output.Quote(req.Source))
	}
	for _, file := range req.Files {
		words = append(words, output.FlagWords(output.FlagValue{Name: "files", Value: file})...)
	}
	words = append(words, output.FlagWords(output.FlagValue{Name: "to", Value: req.To})...)
	words = append(words, switched("ff", req.FastForward)...)
	words = append(words, output.FlagWords(output.FlagValue{Name: "from", Value: req.From})...)
	words = append(words, output.FlagWords(output.FlagValue{Name: "isolation", Value: string(req.Isolation)})...)
	words = append(words, switched("keep", req.Keep)...)
	words = append(words, output.FlagWords(output.FlagValue{Name: "on-conflict", Value: string(req.OnConflict)})...)
	for _, answer := range req.Remember {
		words = append(words, output.FlagWords(output.FlagValue{Name: "remember", Value: answer})...)
	}
	if req.Ask {
		words = append(words, "--ask")
	}
	return strings.Join(words, " ")
}

func switched(name string, value *bool) []string {
	switch {
	case value == nil:
		return nil
	case *value:
		return []string{"--" + name}
	}
	return []string{"--" + name + "=false"}
}
