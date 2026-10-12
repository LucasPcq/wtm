package git_test

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra/git"
)

func TestARepoReadsItsDirAndNotTheGitDirItInherits(t *testing.T) {
	repo := newRepo(t)
	other := newRepo(t)
	t.Setenv(domain.EnvGitDir, other.Dir+"/.git")
	t.Setenv(domain.EnvGitWorkTree, other.Dir)

	main, err := repo.MainWorktree(t.Context())

	require.NoError(t, err)
	assert.Equal(t, repo.Dir, main.Path)
}

func TestARepoWithNoDirReadsTheCurrentDirectory(t *testing.T) {
	repo := newRepo(t)
	t.Chdir(repo.Dir)

	main, err := git.Repo{Remote: remote}.MainWorktree(t.Context())

	require.NoError(t, err)
	assert.Equal(t, repo.Dir, main.Path)
}

func TestAnErrorCarriesTheCallAndWhatGitSaid(t *testing.T) {
	t.Setenv("LC_ALL", "C")
	dir := t.TempDir()

	_, err := git.Repo{Dir: dir, Remote: remote}.Worktrees(t.Context())

	var gitErr *git.Error
	require.ErrorAs(t, err, &gitErr)
	assert.Equal(t, []string{"-C", dir, "worktree", "list", "--porcelain", "-z"}, gitErr.Args)
	assert.Equal(t, 128, gitErr.Exit)
	assert.Contains(t, gitErr.Stderr, "not a git repository")
	assert.Contains(t, err.Error(), "git -C "+dir+" worktree list")
	var exit *exec.ExitError
	assert.ErrorAs(t, err, &exit)
}

func TestAnErrorWithoutAnExitStatusSaysWhy(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	_, err := git.Repo{Dir: t.TempDir(), Remote: remote}.Worktrees(t.Context())

	var gitErr *git.Error
	require.ErrorAs(t, err, &gitErr)
	assert.Equal(t, -1, gitErr.Exit)
	assert.Contains(t, err.Error(), "executable file not found")
}
