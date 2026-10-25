package form

import (
	"cmp"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

var skipped = map[domain.Skip]string{
	domain.SkipFlagOnly:        "set by its flag only",
	domain.SkipExists:          "the target worktree exists: nothing is created",
	domain.SkipNotBehind:       "nothing to fast-forward: the source is not behind origin",
	domain.SkipHeld:            "nothing to fast-forward: %s holds the changes being extracted",
	domain.SkipRunUnread:       "run.toml could not be read",
	domain.SkipNothingIsolated: "run.toml declares nothing a worktree isolates",
	domain.SkipNoConflict:      "nothing conflicts with the target",
	domain.SkipAll:             "--all reads every worktree",
}

// Skipped is why a step of form is not asked, as the wizard's trail words it; empty when it is asked. The held
// fast-forward names the branch holding the changes: extract's answer to its source field, which the code does not
// carry.
func Skipped(skip domain.Skip, form domain.Form) string {
	if skip != domain.SkipHeld {
		return skipped[skip]
	}

	return strings.Replace(skipped[skip], "%s", cmp.Or(answerOf(form, "source"), "the source"), 1)
}

func answerOf(form domain.Form, path string) string {
	for _, field := range form.Fields {
		if answer := field.Answer(); field.Spec().Path == path && len(answer) > 0 {
			return answer[0]
		}
	}

	return ""
}
