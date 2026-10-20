package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheEnvResolvesTheWorkingAndProjectDirectories(t *testing.T) {
	dir, named := t.TempDir(), t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(named, link))
	t.Chdir(dir)
	t.Setenv(projectDirEnv, link)
	t.Setenv(CorrelationIDEnv, "corr-1")

	assert.Equal(t, Env{Cwd: realPath(t, dir), ProjectDir: realPath(t, named), Named: true, CorrelationID: "corr-1"}, ReadEnv())
}

func TestWithoutAProjectDirTheProjectIsOpenedFromTheWorkingDirectoryAsGiven(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv(projectDirEnv, "")

	cwd, err := os.Getwd()
	require.NoError(t, err)
	assert.Equal(t, cwd, ReadEnv().ProjectDir)
	assert.False(t, ReadEnv().Named)
}

func TestAnUnreadablePathStaysEmpty(t *testing.T) {
	assert.Empty(t, resolved(""))
}
