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

// — What git worktree list shows —

func TestWorktreesListsTheMainWorktreeFirstOnItsBranch(t *testing.T) {
	repo := newRepo(t)
	linked := filepath.Join(tempDir(t), "feature")
	gittest.Git(t, repo.Dir, "worktree", "add", "-b", "feature", linked)

	worktrees, err := repo.Worktrees(t.Context())

	require.NoError(t, err)
	assert.Equal(t, []git.Worktree{
		{Path: repo.Dir, Head: head(t, repo.Dir, "main"), Main: true, State: git.OnBranch{Branch: "main"}},
		{Path: linked, Head: head(t, repo.Dir, "feature"), State: git.OnBranch{Branch: "feature"}},
	}, worktrees)
}

func TestWorktreesKeepsAPathWithASpace(t *testing.T) {
	repo := newRepo(t)
	linked := filepath.Join(tempDir(t), "with a space")
	gittest.Git(t, repo.Dir, "worktree", "add", "-b", "spaced", linked)

	worktrees, err := repo.Worktrees(t.Context())

	require.NoError(t, err)
	assert.Equal(t, []string{repo.Dir, linked}, paths(worktrees))
}

func TestWorktreesReadsADetachedHead(t *testing.T) {
	repo := newRepo(t)
	linked := filepath.Join(tempDir(t), "detached")
	gittest.Git(t, repo.Dir, "worktree", "add", "--detach", linked)

	worktrees, err := repo.Worktrees(t.Context())

	require.NoError(t, err)
	require.Len(t, worktrees, 2)
	assert.Equal(t, git.Detached{}, worktrees[1].State)
	assert.Equal(t, head(t, repo.Dir, "main"), worktrees[1].Head)
}

func TestWorktreesMarksALockedAndAPrunableWorktree(t *testing.T) {
	repo := newRepo(t)
	locked := filepath.Join(tempDir(t), "locked")
	gone := filepath.Join(tempDir(t), "gone")
	gittest.Git(t, repo.Dir, "worktree", "add", "-b", "locked", locked)
	gittest.Git(t, repo.Dir, "worktree", "lock", "--reason", "on a stick", locked)
	gittest.Git(t, repo.Dir, "worktree", "add", "-b", "gone", gone)
	require.NoError(t, os.RemoveAll(gone))

	worktrees, err := repo.Worktrees(t.Context())

	require.NoError(t, err)
	require.Len(t, worktrees, 3)
	assert.True(t, worktrees[1].Locked)
	assert.False(t, worktrees[1].Prunable)
	assert.True(t, worktrees[2].Prunable)
	assert.Equal(t, git.OnBranch{Branch: "gone"}, worktrees[2].State)
}

func TestWorktreesReadsABareRepository(t *testing.T) {
	bare := resolved(t, t.TempDir())
	gittest.Git(t, bare, "init", "--bare", "-b", "main")

	worktrees, err := git.Repo{Dir: bare, Remote: remote}.Worktrees(t.Context())

	require.NoError(t, err)
	assert.Equal(t, []git.Worktree{{Path: bare, Main: true, State: git.Bare{}}}, worktrees)
}

// — A rebase in progress: HEAD is detached, the branch is in the admin dir —

func TestWorktreesReadsTheBranchOfARebaseInProgress(t *testing.T) {
	cases := []struct {
		name    string
		backend string
		linked  bool
	}{
		{name: "merge backend, in a linked worktree", backend: "--merge", linked: true},
		{name: "apply backend, in the main worktree", backend: "--apply", linked: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := newRepo(t)
			commitFile(t, repo.Dir, "file", "base")
			gittest.CreateBranch(t, repo.Dir, "feature")
			commitFile(t, repo.Dir, "file", "main")
			where := checkOutFeature(t, repo.Dir, c.linked)
			commitFile(t, where, "file", "feature")
			rebase(t, where, c.backend)

			worktrees, err := repo.Worktrees(t.Context())

			require.NoError(t, err)
			rebasing := worktrees[len(worktrees)-1]
			assert.Equal(t, where, rebasing.Path)
			assert.Equal(t, git.Rebasing{Branch: "feature"}, rebasing.State)
		})
	}
}

// — The main worktree, from anywhere —

func TestMainWorktreeIsTheFirstEvenSeenFromALinkedOne(t *testing.T) {
	repo := newRepo(t)
	linked := filepath.Join(tempDir(t), "feature")
	gittest.Git(t, repo.Dir, "worktree", "add", "-b", "feature", linked)

	main, err := git.Repo{Dir: linked, Remote: remote}.MainWorktree(t.Context())

	require.NoError(t, err)
	assert.Equal(t, repo.Dir, main.Path)
	assert.True(t, main.Main)
}

func TestWorktreesFailsOutsideARepository(t *testing.T) {
	repo := git.Repo{Dir: t.TempDir(), Remote: remote}

	_, err := repo.Worktrees(t.Context())
	_, mainErr := repo.MainWorktree(t.Context())

	var gitErr *git.Error
	require.ErrorAs(t, err, &gitErr)
	assert.Equal(t, 128, gitErr.Exit)
	require.ErrorAs(t, mainErr, &gitErr)
}
