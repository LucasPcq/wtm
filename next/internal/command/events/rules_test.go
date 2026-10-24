package events_test

import (
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/command/events"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
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

func TestAPollReadsEachWorktreesHeadCheckoutAndJobs(t *testing.T) {
	facts := events.Facts{Repository: worktree.Repository{Snapshot: worktree.Snapshot{
		Worktrees: []worktree.Worktree{
			{Path: "/repo", Main: true, State: worktree.OnBranch{Branch: "main"}},
			{Path: "/trees/a", State: worktree.Rebasing{Branch: "feat/a"}},
			{Path: "/trees/detached", State: worktree.Detached{}},
		},
		Branches: worktree.Branches{Local: []worktree.Branch{{Name: "main", Tip: "m1"}, {Name: "feat/a", Tip: "a1"}}},
	}}}
	jobs := []run.Job{{Name: "web", WorkDir: "/trees/a", State: run.Running}, {Name: "api", WorkDir: "/trees/a", State: run.Crashed}}

	polled := events.PollOf(facts, jobs, true)
	unknown := events.PollOf(facts, nil, false)

	assert.Equal(t, []string{"feat/a", "main"}, slices.Sorted(maps.Keys(polled)))
	assert.Equal(t, "a1", polled["feat/a"].Head)
	assert.Equal(t, "rebasing", polled["feat/a"].Checkout)
	assert.Equal(t, []string{"api=crashed", "web=running"}, polled["feat/a"].Jobs)
	assert.Equal(t, []string{}, polled["main"].Jobs)
	assert.Equal(t, "feat/a", polled["feat/a"].Identity.Branch)
	assert.Nil(t, unknown["feat/a"].Jobs)
}

func TestChangesNameWhatMovedInEachWorktree(t *testing.T) {
	before := events.Polled{
		"feat/a": {Identity: worktree.Identity{Branch: "feat/a"}, Head: "a1", Checkout: "branch", Jobs: []string{}},
		"feat/b": {Identity: worktree.Identity{Branch: "feat/b"}, Head: "b1", Checkout: "branch", Jobs: []string{}},
	}

	for name, c := range map[string]struct {
		now     events.Polled
		changed []domain.WorktreeChanged
		reset   bool
	}{
		"nothing": {now: before},
		"head_and_jobs": {
			now: events.Polled{
				"feat/a": {Identity: worktree.Identity{Branch: "feat/a"}, Head: "a2", Checkout: "branch", Jobs: []string{"web=running"}},
				"feat/b": before["feat/b"],
			},
			changed: []domain.WorktreeChanged{{Worktree: worktree.Identity{Branch: "feat/a"}, Changed: []worktree.PolledField{worktree.PolledHead, worktree.PolledJobs}}},
		},
		"checkout": {
			now: events.Polled{
				"feat/a": before["feat/a"],
				"feat/b": {Identity: worktree.Identity{Branch: "feat/b"}, Head: "b1", Checkout: "rebasing", Jobs: []string{}},
			},
			changed: []domain.WorktreeChanged{{Worktree: worktree.Identity{Branch: "feat/b"}, Changed: []worktree.PolledField{worktree.PolledCheckout}}},
		},
		"jobs_unknown": {
			now: events.Polled{
				"feat/a": {Identity: worktree.Identity{Branch: "feat/a"}, Head: "a1", Checkout: "branch"},
				"feat/b": before["feat/b"],
			},
		},
		"appeared": {
			now:     events.Polled{"feat/a": before["feat/a"], "feat/b": before["feat/b"], "feat/c": {Identity: worktree.Identity{Branch: "feat/c"}}},
			changed: []domain.WorktreeChanged{{Worktree: worktree.Identity{Branch: "feat/c"}, Changed: []worktree.PolledField{worktree.PolledHead, worktree.PolledCheckout, worktree.PolledJobs}}},
		},
		"went_away": {now: events.Polled{"feat/a": before["feat/a"]}, reset: true},
	} {
		t.Run(name, func(t *testing.T) {
			changed, reset := events.Changes(before, c.now)

			assert.Equal(t, c.changed, changed)
			assert.Equal(t, c.reset, reset)
		})
	}
}

func TestJobsUnknownAreCarriedFromTheLastPoll(t *testing.T) {
	before := events.Polled{"feat/a": {Head: "a1", Jobs: []string{"web=running"}}}

	carried := events.Carried(before, events.Polled{"feat/a": {Head: "a2"}})

	assert.Equal(t, events.Polled{"feat/a": {Head: "a2", Jobs: []string{"web=running"}}}, carried)
}
