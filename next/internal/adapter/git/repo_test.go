package git_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/git"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/wtm/branch"
	"github.com/LucasPcq/wtm/next/internal/wtm/worktree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestARepoReadsItsDirAndNotTheGitDirItInherits(t *testing.T) {
	repo := newRepo(t)
	other := newRepo(t)
	t.Setenv("GIT_DIR", other.Dir+"/.git")
	t.Setenv("GIT_WORK_TREE", other.Dir)

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

func TestAnInterruptedCallIsTheContextsError(t *testing.T) {
	repo, _ := newRepoWithOrigin(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := repo.FetchRef(ctx, "main")

	var gitErr *git.Error
	require.ErrorAs(t, err, &gitErr)
	assert.Equal(t, -1, gitErr.Exit)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestAGitChildRunsInAProcessGroupOfItsOwn(t *testing.T) {
	repo := newRepo(t)
	recorded := filepath.Join(t.TempDir(), "pgid")
	hook := filepath.Join(repo.Dir, ".git", "hooks", "post-checkout")
	require.NoError(t, os.WriteFile(hook, []byte("#!/bin/sh\nps -o pgid= -p $$ > "+recorded+"\n"), 0o755))

	err := repo.AddWorktree(t.Context(), worktree.Addition{Path: filepath.Join(gittest.TempDir(t), "topic"), Checkout: worktree.NewBranch{Name: "topic", From: branch.LocalBranch{Name: "main"}}})

	require.NoError(t, err)
	content, err := os.ReadFile(recorded)
	require.NoError(t, err)
	pgid, err := strconv.Atoi(strings.TrimSpace(string(content)))
	require.NoError(t, err)
	assert.NotEqual(t, syscall.Getpgrp(), pgid)
}

func TestAFolderOutsideAnyRepositoryIsNoRepository(t *testing.T) {
	_, err := git.Repo{Dir: t.TempDir()}.MainWorktree(t.Context())

	assert.ErrorIs(t, err, git.ErrNotRepository)
}
