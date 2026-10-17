package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheProjectIsTheMainWorktreeOfWhereverItRuns(t *testing.T) {
	main := gittest.Resolved(t, gittest.InitRepo(t))
	linked := filepath.Join(gittest.TempDir(t), "linked")
	gittest.Git(t, main, "worktree", "add", "-b", "feat/x", linked)
	t.Setenv(projectDirEnv, "")
	t.Setenv(stateDirEnv, "")

	params, err := locate(t.Context(), linked)

	require.NoError(t, err)
	assert.Equal(t, wireParams{MainPath: main, StateDir: filepath.Join(main, ".git", "wtm"), CommonDir: filepath.Join(main, ".git")}, params)
}

func TestTheProjectAndStateDirsCanBeNamed(t *testing.T) {
	main := gittest.Resolved(t, gittest.InitRepo(t))
	symlinked := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(main, symlinked))
	t.Setenv(projectDirEnv, symlinked)
	t.Setenv(stateDirEnv, "/state")

	params, err := locate(t.Context(), t.TempDir())

	require.NoError(t, err)
	assert.Equal(t, wireParams{MainPath: main, StateDir: "/state", CommonDir: filepath.Join(main, ".git")}, params)
}

func TestAFolderOutsideAnyRepositoryIsRefused(t *testing.T) {
	outside := t.TempDir()
	t.Setenv(projectDirEnv, "")

	_, err := locate(t.Context(), outside)

	failure, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, domain.CodeNotARepository, failure.Code)
	assert.Equal(t, outside+" is not a git repository", failure.Message)
}

func TestANamedProjectDirOutsideAnyRepositoryIsRefused(t *testing.T) {
	named := t.TempDir()
	t.Setenv(projectDirEnv, named)

	_, err := locate(t.Context(), t.TempDir())

	failure, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, domain.CodeNotARepository, failure.Code)
	assert.Equal(t, gittest.Resolved(t, named)+" is not a git repository", failure.Message)
}

func TestACorrelationIDWithAControlCharacterIsAUsageError(t *testing.T) {
	t.Setenv(correlationIDEnv, "bad\x01id")

	_, err := open(t.Context())

	failure, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, domain.KindInvalid, failure.Kind)
	assert.Equal(t, "$WTM_CORRELATION_ID contains a control character: invalid correlation id", failure.Message)
}
