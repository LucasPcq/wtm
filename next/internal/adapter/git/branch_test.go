package git_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/git"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/wtm/branch"
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
	assert.Equal(t, branch.Branches{
		Local:  []branch.Branch{{Name: "feature", Tip: main}, {Name: "local-only", Tip: main}, {Name: "main", Tip: main}},
		Remote: []branch.Branch{{Name: "feature", Tip: main}, {Name: "main", Tip: main}},
	}, branches)
}

func TestBranchesFollowsTheRemoteItIsGiven(t *testing.T) {
	repo := newRepo(t)
	gittest.Git(t, repo.Dir, "update-ref", "refs/remotes/upstream/feature", "main")
	gittest.Git(t, repo.Dir, "update-ref", "refs/remotes/origin/main", "main")
	repo.Remote = "upstream"

	branches, err := repo.Branches(t.Context())

	require.NoError(t, err)
	assert.Equal(t, []string{"feature"}, gittest.Names(branches.Remote))
}

func TestAheadBehindCountsBothSides(t *testing.T) {
	repo := newRepo(t)
	gittest.CreateBranch(t, repo.Dir, "feature")
	commitFile(t, repo.Dir, "main-only", "1")
	gittest.Git(t, repo.Dir, "checkout", "feature")
	commitFile(t, repo.Dir, "feature-only", "1")
	commitFile(t, repo.Dir, "feature-only", "2")

	counts, err := repo.AheadBehind(t.Context(), branch.AheadBehindParams{Of: branch.LocalBranch{Name: "feature"}, Against: branch.LocalBranch{Name: "main"}})

	require.NoError(t, err)
	assert.Equal(t, branch.AheadBehind{Ahead: 2, Behind: 1}, counts)
}

func TestAheadBehindAgainstTheRemoteBranch(t *testing.T) {
	repo, origin := newRepoWithOrigin(t)
	commitFile(t, repo.Dir, "local", "1")
	advanceOnOrigin(t, origin, "main")
	require.NoError(t, repo.FetchRef(t.Context(), "main"))

	counts, err := repo.AheadBehind(t.Context(), branch.AheadBehindParams{Of: branch.LocalBranch{Name: "main"}, Against: branch.RemoteBranch{Name: "main"}})

	require.NoError(t, err)
	assert.Equal(t, branch.AheadBehind{Ahead: 1, Behind: 1}, counts)
}

func TestAheadBehindFailsOnAnUnknownRefRatherThanCountZero(t *testing.T) {
	repo := newRepo(t)
	main := branch.LocalBranch{Name: "main"}

	_, unknownErr := repo.AheadBehind(t.Context(), branch.AheadBehindParams{Of: main, Against: branch.RemoteBranch{Name: "main"}})
	_, noOfErr := repo.AheadBehind(t.Context(), branch.AheadBehindParams{Against: main})
	_, noAgainstErr := repo.AheadBehind(t.Context(), branch.AheadBehindParams{Of: main})

	var gitErr *git.Error
	require.ErrorAs(t, unknownErr, &gitErr)
	require.Error(t, noOfErr)
	require.Error(t, noAgainstErr)
}

func TestDeleteBranchDropsABranchEvenUnmerged(t *testing.T) {
	repo := newRepo(t)
	gittest.Git(t, repo.Dir, "checkout", "-b", "unmerged")
	commitFile(t, repo.Dir, "work", "1")
	gittest.Git(t, repo.Dir, "checkout", "main")

	require.NoError(t, repo.DeleteBranch(t.Context(), "unmerged"))

	assert.False(t, has(t, repo, branch.LocalBranch{Name: "unmerged"}))
}

func TestTheFakeAndGitAgreeOnTheTipsAndCountsOfTheirBranches(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			main := branch.LocalBranch{Name: "main"}

			assert.True(t, has(t, impl.port, branch.LocalBranch{Name: "feature"}))
			assert.False(t, has(t, impl.port, branch.LocalBranch{Name: "remote-only"}))
			assert.True(t, has(t, impl.port, branch.RemoteBranch{Name: "remote-only"}))
			assert.False(t, has(t, impl.port, branch.RemoteBranch{Name: "pushed"}))
			assert.Equal(t, branch.AheadBehind{}, aheadBehind(t, impl.port, branch.AheadBehindParams{Of: main, Against: branch.RemoteBranch{Name: "main"}}))
			assert.Equal(t, branch.AheadBehind{Ahead: 1}, aheadBehind(t, impl.port, branch.AheadBehindParams{Of: branch.LocalBranch{Name: "unmerged"}, Against: main}))
			assert.Equal(t, branch.AheadBehind{Ahead: 1, Behind: 1}, aheadBehind(t, impl.port, branch.AheadBehindParams{Of: branch.LocalBranch{Name: "linked"}, Against: branch.LocalBranch{Name: "unmerged"}}))
			assert.Equal(t, tip(t, impl.port, main), tip(t, impl.port, branch.RemoteBranch{Name: "feature"}))
			assert.NotEqual(t, tip(t, impl.port, main), tip(t, impl.port, branch.LocalBranch{Name: "unmerged"}))
			assert.NotEqual(t, tip(t, impl.port, branch.LocalBranch{Name: "diverged"}), tip(t, impl.port, branch.RemoteBranch{Name: "diverged"}))
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
			_, err := impl.port.AheadBehind(t.Context(), branch.AheadBehindParams{Of: branch.LocalBranch{Name: "main"}, Against: branch.RemoteBranch{Name: "nope"}})
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
