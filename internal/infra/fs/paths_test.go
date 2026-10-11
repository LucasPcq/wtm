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

func TestGlobalDirIsUnderTheUserConfigDirectory(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	configDir, err := os.UserConfigDir()
	require.NoError(t, err)

	dir, err := fs.GlobalDir()

	require.NoError(t, err)
	assert.Equal(t, configDir, filepath.Dir(dir))
}
