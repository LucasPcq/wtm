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
			before := snapshot(t, impl.port)
			params := gitwt.Params{Git: impl.port, Path: filepath.Join(tempDir(t), "topic"), Checkout: git.NewBranch{Name: "topic", From: git.RemoteBranch{Name: "main"}}}

			require.NoError(t, gitwt.Add(ctx, params))
			added := snapshot(t, impl.port)
			untracked(t, params.Path)
			require.NoError(t, gitwt.Remove(ctx, params))

			assert.Equal(t, append(before.worktrees, git.Worktree{Path: params.Path, State: git.OnBranch{Branch: "topic"}}), added.worktrees)
			assert.Equal(t, []string{"feature", "main", "topic"}, added.branches.Local)
			assert.Equal(t, before, snapshot(t, impl.port))
		})
	}
}

func TestTheRemoveOfAnExistingBranchKeepsTheBranch(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			ctx := t.Context()
			before := snapshot(t, impl.port)
			params := gitwt.Params{Git: impl.port, Path: filepath.Join(tempDir(t), "feature"), Checkout: git.ExistingBranch{Name: "feature"}}

			require.NoError(t, gitwt.Add(ctx, params))
			untracked(t, params.Path)
			require.NoError(t, gitwt.Remove(ctx, params))

			assert.Equal(t, before, snapshot(t, impl.port))
		})
	}
}

func TestARefusedAddChangesNothing(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			before := snapshot(t, impl.port)

			err := gitwt.Add(t.Context(), gitwt.Params{Git: impl.port, Path: filepath.Join(tempDir(t), "main"), Checkout: git.ExistingBranch{Name: "main"}})

			require.Error(t, err)
			assert.Equal(t, before, snapshot(t, impl.port))
		})
	}
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
