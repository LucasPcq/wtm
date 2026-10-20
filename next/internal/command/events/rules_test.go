package events_test

import (
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/command/events"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/stretchr/testify/assert"
)

func TestASnapshotNamesEachWorktreeOnABranchByItsRecord(t *testing.T) {
	facts := events.Facts{
		Repository: worktree.Repository{Snapshot: worktree.Snapshot{Worktrees: []worktree.Worktree{
			{Path: "/repo", Main: true, State: worktree.OnBranch{Branch: "main"}},
			{Path: "/trees/b", State: worktree.OnBranch{Branch: "feat/b"}},
			{Path: "/trees/a", State: worktree.Rebasing{Branch: "feat/a"}},
			{Path: "/trees/detached", State: worktree.Detached{}},
			{Path: "/trees/new", State: worktree.OnBranch{Branch: "feat/new"}},
		}}},
		Metas: map[string]worktree.Meta{
			"feat/a": {SourceBranch: "main", CreatedAt: "t1", Ordinal: 2, Isolation: project.Verbatim},
			"feat/b": {SourceBranch: "feat/a", CreatedAt: "t2", Ordinal: 2},
		},
	}

	assert.Equal(t, []worktree.Identity{
		{Branch: "main", Path: "/repo", Ordinal: new(0), Isolation: project.Isolated, Main: true},
		// feat/a sorts first: the ordinal both claim is feat/a's
		{Branch: "feat/b", Path: "/trees/b", Parent: "feat/a", Isolation: project.Isolated, CreatedAt: "t2"},
		{Branch: "feat/a", Path: "/trees/a", Parent: "main", Ordinal: new(2), Isolation: project.Verbatim, CreatedAt: "t1"},
		{Branch: "feat/new", Path: "/trees/new", Isolation: project.Isolated},
	}, events.Identities(facts))
}

func TestASnapshotOfNoWorktreeIsAnEmptyList(t *testing.T) {
	assert.Equal(t, []worktree.Identity{}, events.Identities(events.Facts{}))
}

// Legacy waited 400ms before its first attempt to subscribe again.
func TestTheFirstWaitBeforeSubscribingAgainIsTheShortestThenItDoublesUpToALimit(t *testing.T) {
	var backoff events.Backoff
	var waits []time.Duration
	for range 7 {
		waits = append(waits, backoff.Next(false))
	}

	assert.Equal(t, []time.Duration{200 * time.Millisecond, 400 * time.Millisecond, 800 * time.Millisecond, 1600 * time.Millisecond, 3200 * time.Millisecond, 5 * time.Second, 5 * time.Second}, waits)
	assert.Equal(t, 200*time.Millisecond, backoff.Next(true), "an attempt that reached ready starts the waits over")
}
