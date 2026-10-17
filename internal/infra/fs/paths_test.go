package fs_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/infra/fs"
)

func TestScratchLockPathIsOneLockPerFileOutsideItsDirectory(t *testing.T) {
	dir := t.TempDir()
	env := filepath.Join(dir, ".env")

	lock := fs.ScratchLockPath(env)

	assert.Equal(t, lock, fs.ScratchLockPath(env), "two processes must meet on the same lock")
	assert.NotEqual(t, lock, fs.ScratchLockPath(filepath.Join(dir, "compose.yml")))
	assert.True(t, strings.HasPrefix(lock, os.TempDir()))
	assert.NotEqual(t, dir, filepath.Dir(lock))
}

func TestScratchLockPathIsTheSameThroughASymlinkedDirectory(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real")
	require.NoError(t, os.Mkdir(real, 0o755))
	link := filepath.Join(dir, "link")
	require.NoError(t, os.Symlink(real, link))

	assert.Equal(t, fs.ScratchLockPath(filepath.Join(real, ".env")), fs.ScratchLockPath(filepath.Join(link, ".env")), "absent file")
	writeFixture(t, filepath.Join(real, ".env"), "")
	assert.Equal(t, fs.ScratchLockPath(filepath.Join(real, ".env")), fs.ScratchLockPath(filepath.Join(link, ".env")), "present file")
}

func TestGlobalDirIsUnderTheUserConfigDirectory(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	configDir, err := os.UserConfigDir()
	require.NoError(t, err)

	dir, err := fs.GlobalDir()

	require.NoError(t, err)
	assert.Equal(t, configDir, filepath.Dir(dir))
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
