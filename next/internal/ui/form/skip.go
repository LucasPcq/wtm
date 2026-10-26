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
	domain.SkipNoOrphan:        "no child worktree is left behind",
	domain.SkipSafe:            "every worktree is safe to remove",
}

// Skipped is how the wizard's trail words why a step is not asked, empty when it has no words for it; a command may
// word a reason its own way (Command.Skipped).
func Skipped(skip domain.Skip) string {
	return skipped[skip]
}
