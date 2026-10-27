package output

import (
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

var blockerWords = map[worktree.Blocker]string{
	worktree.BlockerLocked:          "locked",
	worktree.BlockerDirty:           "uncommitted changes",
	worktree.BlockerUnpushed:        "unpushed commits",
	worktree.BlockerOpenPullRequest: "an open pull request",
	worktree.BlockerJobsRunning:     "jobs running",
}

// Blockers words what makes a worktree unsafe to remove, as the commands that remove one list it.
func Blockers(blockers []worktree.Blocker) string {
	words := make([]string, 0, len(blockers))
	for _, blocker := range blockers {
		words = append(words, blockerWords[blocker])
	}

	return strings.Join(words, ", ")
}
