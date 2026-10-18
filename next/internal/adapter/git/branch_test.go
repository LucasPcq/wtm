package git_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"

	"github.com/LucasPcq/wtm/next/internal/adapter/git"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBranchesSplitsLocalFromRemoteAndDropsTheRemoteHead(t *testing.T) {
	repo, _ := newRepoWithOrigin(t)
	gittest.CreateBranch(t, repo.Dir, "feature")
	gittest.PushBranch(t, repo.Dir, "feature")
	gittest.CreateBranch(t, repo.Dir, "local-only")
	gittest.Git(t, repo.Dir, "remote", "set-head", "origin", "main")
	gittest.Git(t, repo.Dir, "update-ref", "refs/remotes/fork/elsewhere", "main")
	gittest.Git(t, repo.Dir, "update-ref", "refs/remotes/origin-fork/elsewhere", "main")

	branches, err := repo.Branches(t.Context())

	require.NoError(t, err)
	main := gittest.Output(t, repo.Dir, "rev-parse", "main")
	assert.Equal(t, worktree.Branches{
		Local:  []worktree.Branch{{Name: "feature", Tip: main}, {Name: "local-only", Tip: main}, {Name: "main", Tip: main}},
		Remote: []worktree.Branch{{Name: "feature", Tip: main}, {Name: "main", Tip: main}},
	}, branches)
}

func TestAheadBehindAgainstTheRemoteBranch(t *testing.T) {
	repo, origin := newRepoWithOrigin(t)
	commitFile(t, repo.Dir, "local", "1")
	advanceOnOrigin(t, origin, "main")
	require.NoError(t, repo.FetchRef(t.Context(), "main"))

	counts, err := repo.AheadBehind(t.Context(), "main")

	require.NoError(t, err)
	assert.Equal(t, worktree.AheadBehind{Ahead: 1, Behind: 1}, counts)
}

func TestAheadBehindFailsOnAnUnknownRefRatherThanCountZero(t *testing.T) {
	repo := newRepo(t)

	_, err := repo.AheadBehind(t.Context(), "main")

	var gitErr *git.Error
	require.ErrorAs(t, err, &gitErr)
}

func TestDeleteBranchDropsABranchEvenUnmerged(t *testing.T) {
	repo := newRepo(t)
	gittest.Git(t, repo.Dir, "checkout", "-b", "unmerged")
	commitFile(t, repo.Dir, "work", "1")
	gittest.Git(t, repo.Dir, "checkout", "main")

	require.NoError(t, repo.DeleteBranch(t.Context(), "unmerged"))

	assert.False(t, has(t, repo, worktree.LocalBranch{Name: "unmerged"}))
}

func TestTheFakeAndGitAgreeOnTheTipsAndCountsOfTheirBranches(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			main := worktree.LocalBranch{Name: "main"}

			assert.True(t, has(t, impl.port, worktree.LocalBranch{Name: "feature"}))
			assert.False(t, has(t, impl.port, worktree.LocalBranch{Name: "remote-only"}))
			assert.True(t, has(t, impl.port, worktree.RemoteBranch{Name: "remote-only"}))
			assert.False(t, has(t, impl.port, worktree.RemoteBranch{Name: "pushed"}))
			assert.Equal(t, worktree.AheadBehind{}, aheadBehind(t, impl.port, "main"))
			assert.Equal(t, worktree.AheadBehind{Ahead: 1}, aheadBehind(t, impl.port, "diverged"))
			assert.Equal(t, tip(t, impl.port, main), tip(t, impl.port, worktree.RemoteBranch{Name: "feature"}))
			assert.NotEqual(t, tip(t, impl.port, main), tip(t, impl.port, worktree.LocalBranch{Name: "unmerged"}))
			assert.NotEqual(t, tip(t, impl.port, worktree.LocalBranch{Name: "diverged"}), tip(t, impl.port, worktree.RemoteBranch{Name: "diverged"}))
		})
	}
}

func TestTheFakeAndGitRefuseTheSameBranchChanges(t *testing.T) {
	refusals := []struct {
		name string
		call func(t *testing.T, impl implementation) error
	}{
		{name: "deleting a branch checked out", call: func(t *testing.T, impl implementation) error {
			return impl.port.DeleteBranch(t.Context(), "main")
		}},
		{name: "deleting a branch a rebase will move", call: func(t *testing.T, impl implementation) error {
			return impl.port.DeleteBranch(t.Context(), "rebasing")
		}},
		{name: "deleting a branch that does not exist", call: func(t *testing.T, impl implementation) error {
			return impl.port.DeleteBranch(t.Context(), "nope")
		}},
		{name: "counting against an unknown ref", call: func(t *testing.T, impl implementation) error {
			_, err := impl.port.AheadBehind(t.Context(), "unmerged")
			return err
		}},
	}
	for _, impl := range implementations(t) {
		for _, refusal := range refusals {
			t.Run(impl.name+": "+refusal.name, func(t *testing.T) {
				before := gittest.StateOf(t, impl.port)

				err := refusal.call(t, impl)

				require.Error(t, err)
				assert.Equal(t, before, gittest.StateOf(t, impl.port))
			})
		}
	}
}
