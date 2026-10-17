package statefile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAWriteCreatesWithItsPermAndAnExistingFileKeepsItsMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")

	set(t, setParams{Path: path, Content: "a", Perm: 0o600})
	assert.Equal(t, os.FileMode(0o600), modeOf(t, path))

	set(t, setParams{Path: path, Content: "b", Perm: 0o644})
	assert.Equal(t, os.FileMode(0o600), modeOf(t, path))
	assert.Equal(t, "b", contentOf(t, path))
}

func TestAWriteWithoutPermCreatesAReadableFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")

	set(t, setParams{Path: path, Content: "a"})

	assert.Equal(t, os.FileMode(0o644), modeOf(t, path))
}

func TestWritingTheSameContentLeavesTheFileAlone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	writeFixture(t, path, "same")
	before, err := os.Stat(path)
	require.NoError(t, err)

	set(t, setParams{Path: path, Content: "same"})

	after, err := os.Stat(path)
	require.NoError(t, err)
	assert.True(t, os.SameFile(before, after), "an unchanged file was replaced")
}

func TestAWriteReplacesTheFileWholeAndLeavesNoTemporaryFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "meta.json")
	writeFixture(t, path, "old")
	before, err := os.Stat(path)
	require.NoError(t, err)

	set(t, setParams{Path: path, Content: "new"})

	after, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, "new", contentOf(t, path))
	assert.False(t, os.SameFile(before, after), "the new content was written into the old file instead of renamed over it")
	assert.ElementsMatch(t, []string{"meta.json", "meta.json.lock"}, entries(t, dir))
}
