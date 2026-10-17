package cli

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/app/create"
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
	words = appendFlag(words, "env-from", string(req.EnvFrom))
	switch {
	case req.FastForward == nil:
	case *req.FastForward:
		words = append(words, "--ff")
	default:
		words = append(words, "--ff=false")
	}
	words = appendFlag(words, "from", req.From)
	if req.IfNotExists {
		words = append(words, "--if-not-exists")
	}
	words = appendFlag(words, "isolation", string(req.Isolation))
	for _, answer := range req.Remember {
		words = appendFlag(words, "remember", answer)
	}
	if req.Ask {
		words = append(words, "--ask")
	}
	return strings.Join(words, " ")
}

func followUpLine(followUp *wtm.FollowUp) (string, error) {
	var request map[string]any
	if len(followUp.Request) > 0 {
		if err := json.Unmarshal(followUp.Request, &request); err != nil {
			return "", fmt.Errorf("the follow-up %s has no request object: %w", followUp.Command, err)
		}
	}
	words := []string{"wtm", followUp.Command}
	if branches, ok := request["branches"].([]any); ok {
		words = append(words, quoted(texts(branches))...)
	}
	for _, key := range slices.Sorted(maps.Keys(request)) {
		flag := strings.ReplaceAll(key, "_", "-")
		switch value := request[key].(type) {
		case bool:
			if value {
				words = append(words, "--"+flag)
			}
		case string:
			words = appendFlag(words, flag, value)
		case []any:
			if key == "branches" {
				continue
			}
			for _, item := range texts(value) {
				words = appendFlag(words, flag, item)
			}
		}
	}
	return strings.Join(words, " "), nil
}

func appendFlag(words []string, flag, value string) []string {
	if value == "" {
		return words
	}
	return append(words, "--"+flag, quote(value))
}

func texts(values []any) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, fmt.Sprint(value))
	}
	return out
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
