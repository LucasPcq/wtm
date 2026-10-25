package form

import "github.com/LucasPcq/wtm/next/internal/domain"

var skipped = map[domain.Skip]string{
	domain.SkipFlagOnly:        "set by its flag only",
	domain.SkipExists:          "the target worktree exists: nothing is created",
	domain.SkipNotBehind:       "nothing to fast-forward: the source is not behind origin",
	domain.SkipHeld:            "nothing to fast-forward: the source holds the changes being extracted",
	domain.SkipRunUnread:       "run.toml could not be read",
	domain.SkipNothingIsolated: "run.toml declares nothing a worktree isolates",
	domain.SkipNoConflict:      "nothing conflicts with the target",
	domain.SkipAll:             "--all reads every worktree",
}

// Skipped is why a step is not asked, as the wizard's trail words it; empty when it is asked.
func Skipped(skip domain.Skip) string {
	return skipped[skip]
}
