package git_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/infra/git"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

func TestAddWorktreeOnANewBranchStartsFromWithoutTrackingIt(t *testing.T) {
	repo, _ := newRepoWithOrigin(t)
	commitFile(t, repo.Dir, "local", "1")
	path := filepath.Join(gittest.TempDir(t), "topic")

	err := repo.AddWorktree(t.Context(), git.AddWorktreeParams{Path: path, Checkout: git.NewBranch{Name: "topic", From: git.RemoteBranch{Name: "main"}}})

	require.NoError(t, err)
	assert.Equal(t, head(t, repo.Dir, "origin/main"), head(t, path, "HEAD"))
	assert.Equal(t, "topic", gittest.Output(t, path, "branch", "--show-current"))
	assert.Empty(t, upstreamOf(t, repo.Dir, "topic"))
}

func TestAddWorktreeChecksOutAnExistingBranch(t *testing.T) {
	repo := newRepo(t)
	gittest.CreateBranch(t, repo.Dir, "feature")
	path := filepath.Join(gittest.TempDir(t), "feature")

	err := repo.AddWorktree(t.Context(), git.AddWorktreeParams{Path: path, Checkout: git.ExistingBranch{Name: "feature"}})

	require.NoError(t, err)
	assert.Equal(t, "feature", gittest.Output(t, path, "branch", "--show-current"))
}

func TestAddWorktreeFailsWithWhatGitSaid(t *testing.T) {
	repo := newRepo(t)

	checkedOut := repo.AddWorktree(t.Context(), git.AddWorktreeParams{Path: filepath.Join(gittest.TempDir(t), "main"), Checkout: git.ExistingBranch{Name: "main"}})
	noFrom := repo.AddWorktree(t.Context(), git.AddWorktreeParams{Path: filepath.Join(gittest.TempDir(t), "topic"), Checkout: git.NewBranch{Name: "topic"}})
	noCheckout := repo.AddWorktree(t.Context(), git.AddWorktreeParams{Path: filepath.Join(gittest.TempDir(t), "none")})

	var gitErr *git.Error
	require.ErrorAs(t, checkedOut, &gitErr)
	assert.Contains(t, gitErr.Stderr, "main")
	require.Error(t, noFrom)
	require.Error(t, noCheckout)
}

func TestAddWorktreeRefusesABranchOnlyTheRemoteHasRatherThanCreateOne(t *testing.T) {
	repo, _ := newRepoWithOrigin(t)
	gittest.Git(t, repo.Dir, "push", "origin", "main:remote-only")
	gittest.Git(t, repo.Dir, "fetch", "origin")

	err := repo.AddWorktree(t.Context(), git.AddWorktreeParams{Path: filepath.Join(gittest.TempDir(t), "remote-only"), Checkout: git.ExistingBranch{Name: "remote-only"}})

	require.ErrorIs(t, err, git.ErrNoLocalBranch)
	exists, existsErr := repo.RefExists(t.Context(), git.LocalBranch{Name: "remote-only"})
	require.NoError(t, existsErr)
	assert.False(t, exists)
}

func TestRemoveWorktreeAndDeleteBranchUndoAnAdd(t *testing.T) {
	repo := newRepo(t)
	worktreesBefore, err := repo.Worktrees(t.Context())
	require.NoError(t, err)
	branchesBefore, err := repo.Branches(t.Context())
	require.NoError(t, err)
	path := filepath.Join(gittest.TempDir(t), "topic")
	require.NoError(t, repo.AddWorktree(t.Context(), git.AddWorktreeParams{Path: path, Checkout: git.NewBranch{Name: "topic", From: git.LocalBranch{Name: "main"}}}))

	require.NoError(t, repo.RemoveWorktree(t.Context(), git.RemoveWorktreeParams{Path: path, Force: true}))
	require.NoError(t, repo.DeleteBranch(t.Context(), git.DeleteBranchParams{Name: "topic", Force: true}))

	worktreesAfter, err := repo.Worktrees(t.Context())
	require.NoError(t, err)
	branchesAfter, err := repo.Branches(t.Context())
	require.NoError(t, err)
	assert.Equal(t, worktreesBefore, worktreesAfter)
	assert.Equal(t, branchesBefore, branchesAfter)
	assert.NoDirExists(t, path)
}

func TestRemoveWorktreeKeepsADirtyWorktreeWithoutForce(t *testing.T) {
	repo := newRepo(t)
	path := filepath.Join(gittest.TempDir(t), "dirty")
	require.NoError(t, repo.AddWorktree(t.Context(), git.AddWorktreeParams{Path: path, Checkout: git.NewBranch{Name: "dirty", From: git.LocalBranch{Name: "main"}}}))
	require.NoError(t, os.WriteFile(filepath.Join(path, "untracked"), []byte("work"), 0o644))

	refused := repo.RemoveWorktree(t.Context(), git.RemoveWorktreeParams{Path: path})
	forced := repo.RemoveWorktree(t.Context(), git.RemoveWorktreeParams{Path: path, Force: true})

	var gitErr *git.Error
	require.ErrorAs(t, refused, &gitErr)
	require.NoError(t, forced)
	assert.NoDirExists(t, path)
}

func TestDeleteBranchKeepsAnUnmergedBranchWithoutForce(t *testing.T) {
	repo := newRepo(t)
	gittest.Git(t, repo.Dir, "checkout", "-b", "unmerged")
	commitFile(t, repo.Dir, "work", "1")
	gittest.Git(t, repo.Dir, "checkout", "main")

	refused := repo.DeleteBranch(t.Context(), git.DeleteBranchParams{Name: "unmerged"})
	forced := repo.DeleteBranch(t.Context(), git.DeleteBranchParams{Name: "unmerged", Force: true})

	require.Error(t, refused)
	require.NoError(t, forced)
	exists, err := repo.RefExists(t.Context(), git.LocalBranch{Name: "unmerged"})
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestFetchBringsEveryRemoteBranch(t *testing.T) {
	repo, origin := newRepoWithOrigin(t)
	gittest.CreateBranch(t, repo.Dir, "feature")
	gittest.PushBranch(t, repo.Dir, "feature")
	tips := map[string]string{"main": advanceOnOrigin(t, origin, "main"), "feature": advanceOnOrigin(t, origin, "feature")}

	require.NoError(t, repo.Fetch(t.Context()))

	assert.Equal(t, tips["main"], head(t, repo.Dir, "origin/main"))
	assert.Equal(t, tips["feature"], head(t, repo.Dir, "origin/feature"))
}

func TestFetchRefMovesTheRemoteBranchAndNotTheLocalOne(t *testing.T) {
	repo, origin := newRepoWithOrigin(t)
	local := head(t, repo.Dir, "main")
	tip := advanceOnOrigin(t, origin, "main")

	require.NoError(t, repo.FetchRef(t.Context(), "main"))

	assert.Equal(t, tip, head(t, repo.Dir, "origin/main"))
	assert.Equal(t, local, head(t, repo.Dir, "main"))
}

func TestFastForwardRefMovesABranchCheckedOutNowhere(t *testing.T) {
	repo, origin := newRepoWithOrigin(t)
	gittest.CreateBranch(t, repo.Dir, "feature")
	gittest.PushBranch(t, repo.Dir, "feature")
	tip := advanceOnOrigin(t, origin, "feature")

	from := head(t, repo.Dir, "feature")

	move, err := repo.FastForwardRef(t.Context(), "feature")

	require.NoError(t, err)
	assert.Equal(t, git.Move{From: from, To: tip}, move)
	assert.Equal(t, tip, head(t, repo.Dir, "feature"))
}

func TestFastForwardRefRefusesWhatIsNotAFastForward(t *testing.T) {
	repo, origin := newRepoWithOrigin(t)
	gittest.Git(t, repo.Dir, "checkout", "-b", "diverged")
	gittest.PushBranch(t, repo.Dir, "diverged")
	commitFile(t, repo.Dir, "local", "1")
	gittest.Git(t, repo.Dir, "checkout", "main")
	advanceOnOrigin(t, origin, "diverged")
	advanceOnOrigin(t, origin, "main")

	_, diverged := repo.FastForwardRef(t.Context(), "diverged")
	_, checkedOut := repo.FastForwardRef(t.Context(), "main")

	var gitErr *git.Error
	require.ErrorAs(t, diverged, &gitErr)
	require.ErrorAs(t, checkedOut, &gitErr)
}

func TestMergeFFOnlyMovesTheBranchOfAWorktree(t *testing.T) {
	repo, origin := newRepoWithOrigin(t)
	tip := advanceOnOrigin(t, origin, "main")
	require.NoError(t, repo.FetchRef(t.Context(), "main"))

	from := head(t, repo.Dir, "main")

	move, err := repo.MergeFFOnly(t.Context(), git.MergeFFOnlyParams{Worktree: repo.Dir, Onto: git.RemoteBranch{Name: "main"}})

	require.NoError(t, err)
	assert.Equal(t, git.Move{From: from, To: tip}, move)
	assert.Equal(t, tip, head(t, repo.Dir, "main"))
}

func TestMergeFFOnlyRefusesWhatIsNotAFastForward(t *testing.T) {
	repo, origin := newRepoWithOrigin(t)
	commitFile(t, repo.Dir, "local", "1")
	advanceOnOrigin(t, origin, "main")
	require.NoError(t, repo.FetchRef(t.Context(), "main"))

	_, diverged := repo.MergeFFOnly(t.Context(), git.MergeFFOnlyParams{Worktree: repo.Dir, Onto: git.RemoteBranch{Name: "main"}})
	_, noOnto := repo.MergeFFOnly(t.Context(), git.MergeFFOnlyParams{Worktree: repo.Dir})

	var gitErr *git.Error
	require.ErrorAs(t, diverged, &gitErr)
	require.Error(t, noOnto)
}
