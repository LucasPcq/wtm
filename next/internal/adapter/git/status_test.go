package git_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheFakeAndGitTellADirtyWorktreeFromACleanOne(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			dirty, dirtyErr := impl.port.Dirty(t.Context(), impl.at("dirty"))
			clean, cleanErr := impl.port.Dirty(t.Context(), impl.at("linked"))
			_, notErr := impl.port.Dirty(t.Context(), filepath.Join(impl.root, "taken", "nowhere"))
			_, goneErr := impl.port.Dirty(t.Context(), impl.at("gone"))

			require.NoError(t, dirtyErr)
			assert.True(t, dirty)
			require.NoError(t, cleanErr)
			assert.False(t, clean)
			require.Error(t, notErr)
			require.Error(t, goneErr)
		})
	}
}

func TestTheDiffstatCountsTheStagedFilesAndTheLinesChangedIndexIncluded(t *testing.T) {
	repo := newRepo(t)
	commitFile(t, repo.Dir, "a.txt", "one\ntwo\n")
	require.NoError(t, os.WriteFile(filepath.Join(repo.Dir, "a.txt"), []byte("one\nthree\nfour\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(repo.Dir, "b.txt"), []byte("new\n"), 0o644))
	gittest.Git(t, repo.Dir, "add", "b.txt")

	stat, err := repo.Diffstat(t.Context(), repo.Dir)

	require.NoError(t, err)
	assert.Equal(t, worktree.Diffstat{Staged: 1, Unstaged: 1, Insertions: 3, Deletions: 1}, stat)
}

func TestTheCommitsOfAWorktreeCarryTheirAuthorAndDate(t *testing.T) {
	repo := newRepo(t)

	commits, err := repo.Commits(t.Context(), repo.Dir, 5)

	require.NoError(t, err)
	require.Len(t, commits, 1)
	assert.Equal(t, "initial", commits[0].Subject)
	assert.Equal(t, "Test", commits[0].Author)
	assert.NotEmpty(t, commits[0].SHA)
	assert.False(t, commits[0].At.IsZero())
}
