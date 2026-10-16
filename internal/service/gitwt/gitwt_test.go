package gitwt_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/infra/git"
	"github.com/LucasPcq/wtm/internal/service/gitwt"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

func TestAnAddOnANewBranchAndItsRemoveEndWhereTheyStarted(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			ctx := t.Context()
			before := gittest.StateOf(t, impl.port)
			params := gitwt.Params{Git: impl.port, Path: filepath.Join(impl.root, "topic"), Checkout: git.NewBranch{Name: "topic", From: git.RemoteBranch{Name: "main"}}}

			require.NoError(t, gitwt.Add(ctx, params))
			added := gittest.StateOf(t, impl.port)
			untracked(t, params.Path)
			require.NoError(t, gitwt.Remove(ctx, params))

			assert.Equal(t, append(before.Worktrees, git.Worktree{Path: filepath.Join(gittest.Resolved(t, impl.root), "topic"), State: git.OnBranch{Branch: "topic"}}), added.Worktrees)
			assert.Equal(t, []string{"feature", "gone", "main", "topic"}, gittest.Names(added.Branches.Local))
			assert.Equal(t, before, gittest.StateOf(t, impl.port))
		})
	}
}

func TestTheRemoveOfAnExistingBranchKeepsTheBranch(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			ctx := t.Context()
			before := gittest.StateOf(t, impl.port)
			params := gitwt.Params{Git: impl.port, Path: filepath.Join(impl.root, "feature"), Checkout: git.ExistingBranch{Name: "feature"}}

			require.NoError(t, gitwt.Add(ctx, params))
			untracked(t, params.Path)
			require.NoError(t, gitwt.Remove(ctx, params))

			assert.Equal(t, before, gittest.StateOf(t, impl.port))
		})
	}
}

// The refusals share a repository: each names its own branch, so one left
// behind cannot make the next fail for another reason.
func TestARefusedAddChangesNothing(t *testing.T) {
	refusals := []struct {
		name     string
		folder   string
		checkout git.Checkout
	}{
		{name: "a branch checked out elsewhere", folder: "new", checkout: git.ExistingBranch{Name: "main"}},
		{name: "a branch only the remote has", folder: "new", checkout: git.ExistingBranch{Name: "nope"}},
		{name: "a new branch into a taken folder", folder: "taken", checkout: git.NewBranch{Name: "into-taken", From: git.LocalBranch{Name: "main"}}},
		{name: "a new branch onto a worktree gone", folder: "gone", checkout: git.NewBranch{Name: "onto-gone", From: git.LocalBranch{Name: "main"}}},
		{name: "a new branch that exists, into a taken folder", folder: "taken", checkout: git.NewBranch{Name: "feature", From: git.LocalBranch{Name: "main"}}},
	}
	for _, impl := range implementations(t) {
		for _, refusal := range refusals {
			t.Run(impl.name+": "+refusal.name, func(t *testing.T) {
				before := gittest.StateOf(t, impl.port)

				err := gitwt.Add(t.Context(), gitwt.Params{Git: impl.port, Path: filepath.Join(impl.root, refusal.folder), Checkout: refusal.checkout})

				require.Error(t, err)
				assert.Equal(t, before, gittest.StateOf(t, impl.port))
			})
		}
	}
}

func TestAnAddThatCannotDropTheBranchItLeftSaysSo(t *testing.T) {
	refused := errors.New("refused")
	fake := gittest.NewFake(gittest.FakeParams{Main: "/repo", Branch: "main", Folders: []string{"/wt/topic"}, Fail: map[string]error{"DeleteBranch": refused}})

	err := gitwt.Add(t.Context(), gitwt.Params{Git: fake, Path: "/wt/topic", Checkout: git.NewBranch{Name: "topic", From: git.LocalBranch{Name: "main"}}})

	require.ErrorIs(t, err, refused)
	assert.Contains(t, fake.Calls(), "DeleteBranch topic")
}

func TestARemoveThatCannotRemoveTheWorktreeKeepsTheBranch(t *testing.T) {
	refused := errors.New("refused")
	fake := gittest.NewFake(gittest.FakeParams{Main: "/repo", Branch: "main", Fail: map[string]error{"RemoveWorktree": refused}})
	params := gitwt.Params{Git: fake, Path: "/wt/topic", Checkout: git.NewBranch{Name: "topic", From: git.LocalBranch{Name: "main"}}}
	require.NoError(t, gitwt.Add(t.Context(), params))

	err := gitwt.Remove(t.Context(), params)

	require.ErrorIs(t, err, refused)
	assert.NotContains(t, fake.Calls(), "DeleteBranch topic")
}
