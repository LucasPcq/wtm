package formtest

import (
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

// CommandLine spells a follow-up as a cli test reads it: wtm, the command's words, the worktree's values, then each other answer as its flag, alone when said true, else followed by each value; a link is opened.
func CommandLine(followUp domain.FollowUp) []string {
	if followUp.URL != "" {
		return []string{"open", followUp.URL}
	}

	argv := append([]string{"wtm"}, strings.Fields(followUp.Command)...)
	var flags []string
	for _, answer := range followUp.Answers {
		switch {
		case answer.Path == domain.PathWorktree:
			argv = append(argv, answer.Values...)
		case len(answer.Values) == 1 && answer.Values[0] == "true":
			flags = append(flags, "--"+answer.Path)
		default:
			for _, value := range answer.Values {
				flags = append(flags, "--"+answer.Path, value)
			}
		}
	}
	return append(argv, flags...)
}
