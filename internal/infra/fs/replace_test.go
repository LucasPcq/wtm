package fs_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/infra/fs"
)

func TestReplaceFileWritesItsPermAndTakesNoLock(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "global")
	path := filepath.Join(dir, "jobs.json")

	require.NoError(t, fs.ReplaceFile(fs.ReplaceFileParams{Path: path, Data: []byte(`{"version":2}`), Perm: 0o600}))

	assert.Equal(t, `{"version":2}`, read(t, path))
	assert.Equal(t, os.FileMode(0o600), modeOf(t, path))
	assert.Equal(t, []string{"jobs.json"}, entries(t, dir))
}

func TestReplaceFileReplacesWhatIsThere(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jobs.json")
	writeFixture(t, path, "old")

	require.NoError(t, fs.ReplaceFile(fs.ReplaceFileParams{Path: path, Data: []byte("new")}))

	assert.Equal(t, "new", read(t, path))
}

func TestReplaceFileFailsWhenItsDirectoryCannotBeMade(t *testing.T) {
	notADir := filepath.Join(t.TempDir(), "file")
	writeFixture(t, notADir, "")
	assert.Error(t, fs.ReplaceFile(fs.ReplaceFileParams{Path: filepath.Join(notADir, "jobs.json"), Data: []byte("{}")}))
}
