package output

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

const (
	shellSpecial = " \t\n'\"$`\\|&;<>()*?[]{}~#!"
	ApplyNote    = "apply this plan"
	ReplanNote   = "the plan the facts give now"
)

type FlagValue struct {
	Name  string
	Value string
}

func FlagWords(flag FlagValue) []string {
	if flag.Value == "" {
		return nil
	}
	return []string{"--" + flag.Name, Quote(flag.Value)}
}

func Quoted(words []string) []string {
	out := make([]string, 0, len(words))
	for _, word := range words {
		out = append(out, Quote(word))
	}
	return out
}

func Quote(word string) string {
	if word != "" && !strings.ContainsAny(word, shellSpecial) {
		return word
	}
	return "'" + strings.ReplaceAll(word, "'", `'\''`) + "'"
}

func FollowUpLine(followUp *domain.FollowUp) string {
	var request domain.CleanRequest
	_ = json.Unmarshal(followUp.Request, &request)
	words := append([]string{"wtm", followUp.Command}, Quoted(request.Branches)...)
	if request.Force {
		words = append(words, "--force")
	}
	return strings.Join(words, " ")
}

type Shown struct {
	Cwd  string
	Path string
}

func ShownPath(s Shown) string {
	relative, err := filepath.Rel(s.Cwd, s.Path)
	if err != nil || s.Cwd == "" {
		return s.Path
	}
	return relative
}
