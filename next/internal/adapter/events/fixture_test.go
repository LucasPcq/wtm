package events

import (
	"time"

	"github.com/LucasPcq/wtm/next/internal/wtm/run"
	"github.com/LucasPcq/wtm/next/internal/wtm/worktree"
)

func identity() worktree.Identity {
	return worktree.Identity{Branch: "feat/a", Path: "/repo/.trees/feat-a", Parent: "main", Isolation: run.Isolated, CreatedAt: "2026-10-16T11:00:00Z"}
}

func fixedNow() time.Time {
	return time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
}
