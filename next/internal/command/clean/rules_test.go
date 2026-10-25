package clean_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/clean"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAChildMovesPastEveryParentRemovedToTheBaseBranchAtTheEnd(t *testing.T) {
	facts := clean.Facts{
		Repository: worktree.Repository{
			Config: project.Config{Worktrees: project.Worktrees{BaseBranch: "main"}},
			Snapshot: worktree.Snapshot{Worktrees: []worktree.Worktree{
				{Path: "/main", Main: true, State: worktree.OnBranch{Branch: "main"}},
				{Path: "/a", State: worktree.OnBranch{Branch: "a"}},
				{Path: "/b", State: worktree.OnBranch{Branch: "b"}},
				{Path: "/c", State: worktree.OnBranch{Branch: "c"}},
				{Path: "/d", State: worktree.OnBranch{Branch: "d"}},
			}},
		},
		Records: map[string]worktree.Meta{"a": {SourceBranch: "main"}, "b": {SourceBranch: "a"}, "c": {SourceBranch: "b"}, "d": {SourceBranch: "a"}},
	}

	cases := map[string]struct {
		removed []string
		moves   []clean.Move
	}{
		"onto_grandparent": {removed: []string{"b"}, moves: []clean.Move{{Branch: "c", From: "b", To: "a"}}},
		"past_two":         {removed: []string{"a", "b"}, moves: []clean.Move{{Branch: "d", From: "a", To: "main"}, {Branch: "c", From: "b", To: "main"}}},
		"leaf":             {removed: []string{"c"}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			planned, _, err := clean.Decide(facts, removing(c.removed...))

			require.NoError(t, err)
			assert.Equal(t, c.moves, planned.Moves)
		})
	}
}

func TestTheFingerprintCoversTheHooksTheRemovalRuns(t *testing.T) {
	w := newWorld(t, seed{})
	before, _ := w.plan(t, removing(plain))
	writeFile(t, w.stateDir+"/config.toml", withHook("true"))
	after, _ := w.plan(t, removing(plain))

	first, err := operation.Fingerprint(before.Fingerprinted())
	require.NoError(t, err)
	second, err := operation.Fingerprint(after.Fingerprinted())
	require.NoError(t, err)
	assert.NotEqual(t, first, second)
}

func TestTheRecapNamesWhatAForcedRemovalLosesAndWhereEachChildGoes(t *testing.T) {
	w := newWorld(t, seed{})

	planned, _ := w.plan(t, clean.Request{Worktrees: []string{dirty, parent, "feat/nope"}, Force: true})

	assert.Equal(t, []domain.RecapRow{
		{Key: "remove", Label: "Remove", Path: domain.PathWorktree, Values: []domain.RecapValue{
			{Value: dirty, Note: clean.NoteForced, Params: domain.Params{"blockers": "dirty"}, Text: dirty + " (dirty, forced)"},
			{Value: parent, Text: parent},
		}},
		{Key: "absent", Label: "Already absent", Path: domain.PathWorktree, Values: []domain.RecapValue{{Value: "feat/nope", Note: clean.NoteAbsent, Text: "feat/nope (no worktree: nothing to remove)"}}},
		{Key: "children", Label: "Children", Path: clean.PathReparentChildren, Values: []domain.RecapValue{{Value: child, Note: clean.NoteOrphaned, Params: domain.Params{"from": parent, "to": "main"}, Text: child + " (left on " + parent + ")"}}},
	}, planned.Recap)
}

func TestThePlanLocksEachWorktreeRemovedAndEachChildReparented(t *testing.T) {
	w := newWorld(t, seed{})

	orphaning, _ := w.plan(t, removing(parent))
	reparenting, _ := w.plan(t, clean.Request{Worktrees: []string{parent}, ReparentChildren: true})

	assert.Equal(t, domain.WorktreeLocks([]string{parent}), orphaning.Locks())
	assert.Equal(t, domain.WorktreeLocks([]string{parent, child}), reparenting.Locks())
}
