package formtest

import (
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

// CommandLine spells a follow-up as a cli test reads it: wtm, the command's words, then each answer's values, an answer said true as its flag; a link is opened.
func CommandLine(followUp domain.FollowUp) []string {
	if followUp.URL != "" {
		return []string{"open", followUp.URL}
	}

	argv := append([]string{"wtm"}, strings.Fields(followUp.Command)...)
	var flags []string
	for _, answer := range followUp.Answers {
		if len(answer.Values) == 1 && answer.Values[0] == "true" {
			flags = append(flags, "--"+answer.Path)
			continue
		}
		argv = append(argv, answer.Values...)
	}
	return append(argv, flags...)
}
