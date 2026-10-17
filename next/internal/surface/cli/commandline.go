package cli

import (
	"encoding/json"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/app/create"
	"github.com/LucasPcq/wtm/next/internal/app/provision"
	"github.com/LucasPcq/wtm/next/internal/wtm"
)

const shellSpecial = " \t\n'\"$`\\|&;<>()*?[]{}~#!"

type createLine struct {
	Command string
	Request create.Request
}

func (l createLine) String() string {
	req := l.Request
	words := append([]string{l.Command}, quoted(req.Branches)...)
	words = append(words, flagWords(flagValue{Name: "env-from", Value: string(req.EnvFrom)})...)
	switch {
	case req.FastForward == nil:
	case *req.FastForward:
		words = append(words, "--ff")
	default:
		words = append(words, "--ff=false")
	}
	words = append(words, flagWords(flagValue{Name: "from", Value: req.From})...)
	if req.IfNotExists {
		words = append(words, "--if-not-exists")
	}
	words = append(words, flagWords(flagValue{Name: "isolation", Value: string(req.Isolation)})...)
	for _, answer := range req.Remember {
		words = append(words, flagWords(flagValue{Name: "remember", Value: answer})...)
	}
	if req.Ask {
		words = append(words, "--ask")
	}
	return strings.Join(words, " ")
}

func followUpLine(followUp *wtm.FollowUp) string {
	var request provision.CleanRequest
	_ = json.Unmarshal(followUp.Request, &request)
	words := append([]string{"wtm", followUp.Command}, quoted(request.Branches)...)
	if request.Force {
		words = append(words, "--force")
	}
	return strings.Join(words, " ")
}

type flagValue struct {
	Name  string
	Value string
}

func flagWords(flag flagValue) []string {
	if flag.Value == "" {
		return nil
	}
	return []string{"--" + flag.Name, quote(flag.Value)}
}

func quoted(words []string) []string {
	out := make([]string, 0, len(words))
	for _, word := range words {
		out = append(out, quote(word))
	}
	return out
}

func quote(word string) string {
	if word != "" && !strings.ContainsAny(word, shellSpecial) {
		return word
	}
	return "'" + strings.ReplaceAll(word, "'", `'\''`) + "'"
}
