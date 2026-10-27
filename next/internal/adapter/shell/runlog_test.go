package shell

import (
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEachRunHasItsOwnLogAndTheNextRemovesTheEndedOnes(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "exec", "feat%2Fa")

	ended, first, err := OpenRunLog(dir, "op-1")
	require.NoError(t, err)
	_, _ = io.WriteString(first, "first\n")
	require.NoError(t, first.Close())
	running, second, err := OpenRunLog(dir, "op-2")
	require.NoError(t, err)
	_, _ = io.WriteString(second, "second\n")
	third, last, err := OpenRunLog(dir, "op-3")
	require.NoError(t, err)
	require.NoError(t, last.Close())
	require.NoError(t, second.Close())

	assert.Equal(t, filepath.Join(dir, "op-2.log"), running)
	assert.NoFileExists(t, ended, "an ended run's log goes")
	written, err := os.ReadFile(running)
	require.NoError(t, err)
	assert.Equal(t, "second\n", string(written), "a running one stays whole")
	assert.FileExists(t, third)
}

func TestAPendingLogIsRemovedOnlyOnceStale(t *testing.T) {
	dir := t.TempDir()
	stale, young := filepath.Join(dir, ".op-dead-1"), filepath.Join(dir, ".op-going-2")
	require.NoError(t, os.WriteFile(stale, nil, 0o644))
	require.NoError(t, os.WriteFile(young, nil, 0o644))
	old := time.Now().Add(-2 * pendingStale)
	require.NoError(t, os.Chtimes(stale, old, old))

	_, log, err := OpenRunLog(dir, "op-3")
	require.NoError(t, err)
	require.NoError(t, log.Close())

	assert.NoFileExists(t, stale)
	assert.FileExists(t, young, "a log being put in place now is no run's leftover")
}
