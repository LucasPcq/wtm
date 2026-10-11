package fs_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/infra/fs"
)

func TestFileExists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	assert.False(t, fs.FileExists(path))
	writeFixture(t, path, "")
	assert.True(t, fs.FileExists(path))
}

func TestResolvePathFollowsSymlinksAndKeepsWhatItCannotResolve(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	target := filepath.Join(dir, "target")
	require.NoError(t, os.Mkdir(target, 0o755))
	link := filepath.Join(dir, "link")
	require.NoError(t, os.Symlink(target, link))

	assert.Equal(t, target, fs.ResolvePath(link))
	missing := filepath.Join(dir, "missing")
	assert.Equal(t, missing, fs.ResolvePath(missing))
}
