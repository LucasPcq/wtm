package events

import (
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// identity is the worktree the events of a test are about.
func identity() worktree.Identity {
	return worktree.Identity{Branch: "feat/a", Path: "/repo/.trees/feat-a", Parent: "main", Isolation: project.Isolated, CreatedAt: "2026-10-16T11:00:00Z"}
}

// fixedNow is the clock of a test's publisher.
func fixedNow() time.Time {
	return time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
}
