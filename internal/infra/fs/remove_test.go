package fs_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/infra/fs"
)

func TestRemoveDirDropsTheDirectoryAndWhatItHolds(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "worktrees", "feat-a")
	writeFixture(t, filepath.Join(dir, "meta.json"), "{}")

	require.NoError(t, fs.RemoveDir(dir))

	assert.NoDirExists(t, dir)
	require.NoError(t, fs.RemoveDir(dir), "an absent directory is already removed")
}
