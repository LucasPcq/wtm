package repository_test

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAWorktreeIsPresentDirtyOrMissing(t *testing.T) {
	locked := worktree.Worktree{Path: "/repo/.trees/locked", State: worktree.OnBranch{Branch: "locked"}, Locked: true}
	git := gittest.NewFake(gittest.FakeParams{
		Main: "/repo/main", Branch: "main",
		Worktrees: []worktree.Worktree{linkedAt("dirty"), linkedAt("clean"), linkedAt("gone"), locked},
		Dirty:     []string{"/repo/.trees/dirty"},
		Gone:      []string{"/repo/.trees/gone", locked.Path},
	})
	cases := map[string]struct {
		w        worktree.Worktree
		presence repository.Presence
	}{
		"dirty":       {w: linkedAt("dirty"), presence: repository.Presence{Dirty: true}},
		"clean":       {w: linkedAt("clean")},
		"gone":        {w: linkedAt("gone"), presence: repository.Presence{Missing: true}},
		"prunable":    {w: worktree.Worktree{Path: "/nowhere", Prunable: true}, presence: repository.Presence{Missing: true}},
		"locked_gone": {w: locked, presence: repository.Presence{Missing: true}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			presence, err := repository.Present(t.Context(), git, c.w)

			require.NoError(t, err)
			assert.Equal(t, c.presence, presence)
		})
	}
	assert.NotContains(t, git.Calls(), "Dirty "+locked.Path)
}

func TestAWorktreeGitCannotTellOfFails(t *testing.T) {
	broken := errors.New("git status failed")
	git := gittest.NewFake(gittest.FakeParams{Main: "/repo/main", Branch: "main", Fail: map[string]error{"Dirty": broken}})

	_, err := repository.Present(t.Context(), git, worktree.Worktree{Path: "/repo/main", Main: true})

	assert.ErrorIs(t, err, broken)
}

func TestEachWorktreeIsReadInItsOrder(t *testing.T) {
	var worktrees []worktree.Worktree
	for _, name := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"} {
		worktrees = append(worktrees, linkedAt(name))
	}

	read := repository.Each(worktrees, func(w worktree.Worktree) string { return w.Path })

	assert.Equal(t, "/repo/.trees/a", read[0])
	assert.Equal(t, "/repo/.trees/j", read[9])
	assert.Len(t, read, 10)
}
