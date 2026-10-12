package git_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/infra/git"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

// — Branches —

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
	assert.Equal(t, git.Branches{Local: []string{"feature", "local-only", "main"}, Remote: []string{"feature", "main"}}, branches)
}

func TestBranchesFollowsTheRemoteItIsGiven(t *testing.T) {
	repo := newRepo(t)
	gittest.Git(t, repo.Dir, "update-ref", "refs/remotes/upstream/feature", "main")
	gittest.Git(t, repo.Dir, "update-ref", "refs/remotes/origin/main", "main")
	repo.Remote = "upstream"

	branches, err := repo.Branches(t.Context())

	require.NoError(t, err)
	assert.Equal(t, []string{"feature"}, branches.Remote)
}

// — RefExists: one form, an absence is not an error —

func TestRefExistsTellsAPresentRefFromAnAbsentOne(t *testing.T) {
	repo, _ := newRepoWithOrigin(t)
	gittest.CreateBranch(t, repo.Dir, "local-only")
	cases := []struct {
		name   string
		ref    git.Ref
		exists bool
	}{
		{name: "local branch", ref: git.LocalBranch{Name: "local-only"}, exists: true},
		{name: "absent local branch", ref: git.LocalBranch{Name: "nope"}},
		{name: "remote branch", ref: git.RemoteBranch{Name: "main"}, exists: true},
		{name: "local branch is not on the remote", ref: git.RemoteBranch{Name: "local-only"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			exists, err := repo.RefExists(t.Context(), c.ref)

			require.NoError(t, err)
			assert.Equal(t, c.exists, exists)
		})
	}
}

func TestRefExistsFailsRatherThanAnswerAbsent(t *testing.T) {
	outside := git.Repo{Dir: t.TempDir(), Remote: remote}

	_, outsideErr := outside.RefExists(t.Context(), git.LocalBranch{Name: "main"})
	_, noRefErr := newRepo(t).RefExists(t.Context(), nil)

	var gitErr *git.Error
	require.ErrorAs(t, outsideErr, &gitErr)
	assert.Equal(t, 128, gitErr.Exit)
	require.Error(t, noRefErr)
}

// — AheadBehind —

func TestAheadBehindCountsBothSides(t *testing.T) {
	repo := newRepo(t)
	gittest.CreateBranch(t, repo.Dir, "feature")
	commitFile(t, repo.Dir, "main-only", "1")
	gittest.Git(t, repo.Dir, "checkout", "feature")
	commitFile(t, repo.Dir, "feature-only", "1")
	commitFile(t, repo.Dir, "feature-only", "2")

	counts, err := repo.AheadBehind(t.Context(), git.AheadBehindParams{Of: git.LocalBranch{Name: "feature"}, Against: git.LocalBranch{Name: "main"}})

	require.NoError(t, err)
	assert.Equal(t, git.AheadBehind{Ahead: 2, Behind: 1}, counts)
}

func TestAheadBehindAgainstTheRemoteBranch(t *testing.T) {
	repo, origin := newRepoWithOrigin(t)
	commitFile(t, repo.Dir, "local", "1")
	advanceOnOrigin(t, origin, "main")
	require.NoError(t, repo.FetchRef(t.Context(), "main"))

	counts, err := repo.AheadBehind(t.Context(), git.AheadBehindParams{Of: git.LocalBranch{Name: "main"}, Against: git.RemoteBranch{Name: "main"}})

	require.NoError(t, err)
	assert.Equal(t, git.AheadBehind{Ahead: 1, Behind: 1}, counts)
}

func TestAheadBehindFailsOnAnUnknownRefRatherThanCountZero(t *testing.T) {
	repo := newRepo(t)
	main := git.LocalBranch{Name: "main"}

	_, unknownErr := repo.AheadBehind(t.Context(), git.AheadBehindParams{Of: main, Against: git.RemoteBranch{Name: "main"}})
	_, noOfErr := repo.AheadBehind(t.Context(), git.AheadBehindParams{Against: main})
	_, noAgainstErr := repo.AheadBehind(t.Context(), git.AheadBehindParams{Of: main})

	var gitErr *git.Error
	require.ErrorAs(t, unknownErr, &gitErr)
	require.Error(t, noOfErr)
	require.Error(t, noAgainstErr)
}
