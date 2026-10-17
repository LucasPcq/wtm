package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheNamedProjectIsTheMainWorktreeOfWhicheverWorktreeItNames(t *testing.T) {
	main := gittest.Resolved(t, gittest.InitRepo(t))
	linked := filepath.Join(gittest.TempDir(t), "linked")
	gittest.Git(t, main, "worktree", "add", "-b", "feat/x", linked)
	sub := filepath.Join(main, "sub")
	require.NoError(t, os.Mkdir(sub, 0o755))
	linkedSub := filepath.Join(linked, "sub")
	require.NoError(t, os.Mkdir(linkedSub, 0o755))
	t.Chdir(filepath.Dir(linked))
	t.Setenv(stateDirEnv, "")
	want := wireParams{MainPath: main, StateDir: filepath.Join(main, ".git", "wtm"), CommonDir: filepath.Join(main, ".git")}

	// Named by $WTM_PROJECT_DIR, a linked worktree, a folder in one or a relative path was once taken as the main worktree.
	for _, dir := range []string{main, sub, linked, linkedSub, "linked"} {
		t.Setenv("WTM_PROJECT_DIR", dir)

		params, err := locate(t.Context(), cli.ReadEnv().ProjectDir)

		require.NoError(t, err, dir)
		assert.Equal(t, want, params, dir)
	}
}

func TestTheProjectIsResolvedThroughSymbolicLinksAndItsStateDirCanBeNamed(t *testing.T) {
	main := gittest.Resolved(t, gittest.InitRepo(t))
	symlinked := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(main, symlinked))
	t.Setenv(stateDirEnv, "/state")

	params, err := locate(t.Context(), symlinked)

	require.NoError(t, err)
	assert.Equal(t, wireParams{MainPath: main, StateDir: "/state", CommonDir: filepath.Join(main, ".git")}, params)
}

func TestAFolderOutsideAnyRepositoryIsRefused(t *testing.T) {
	outside := t.TempDir()

	_, err := locate(t.Context(), outside)

	failure, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, domain.CodeNotARepository, failure.Code)
	assert.Equal(t, outside+" is not a git repository", failure.Message)
}

func TestANamedProjectDirOutsideAnyRepositoryIsRefusedByItsResolvedPath(t *testing.T) {
	named := t.TempDir()
	symlinked := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(named, symlinked))
	t.Setenv("WTM_PROJECT_DIR", symlinked)

	_, err := locate(t.Context(), cli.ReadEnv().ProjectDir)

	failure, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, domain.CodeNotARepository, failure.Code)
	assert.Equal(t, gittest.Resolved(t, named)+" is not a git repository", failure.Message)
}
