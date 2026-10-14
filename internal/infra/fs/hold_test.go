package fs_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/infra/fs"
)

func TestAHeldFileIsReadUntilItIsReleased(t *testing.T) {
	dir := t.TempDir()
	held, err := fs.HoldFile(fs.HoldFileParams{Path: filepath.Join(dir, "op-1.json"), Data: []byte(`{"op":1}`)})
	require.NoError(t, err)
	assert.True(t, held.Held)

	assert.Equal(t, [][]byte{[]byte(`{"op":1}`)}, heldFiles(t, dir))

	held.Release()
	assert.Empty(t, heldFiles(t, dir))
	assert.Empty(t, entries(t, dir), "the released file was left behind")
}

func TestAFileNobodyHoldsIsStaleAndRemoved(t *testing.T) {
	dir := t.TempDir()
	stale := filepath.Join(dir, "op-dead.json")
	writeFixture(t, stale, `{"op":"dead"}`)

	assert.Empty(t, heldFiles(t, dir))
	assert.NoFileExists(t, stale)
}

func TestAFileBeingWrittenIsNotRead(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, filepath.Join(dir, ".op-2.json-123"), `{"op":`)

	assert.Empty(t, heldFiles(t, dir))
}

func TestNoDirectoryHoldsNothing(t *testing.T) {
	assert.Empty(t, heldFiles(t, filepath.Join(t.TempDir(), "absent")))
}

func TestAProcessThatDiesHoldsNothing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "op-child.json")
	cmd := exec.Command(os.Args[0], "-test.run=^TestHoldHelperProcess$")
	cmd.Env = append(os.Environ(), envHoldPath+"="+path)
	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)
	defer stdin.Close()
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())
	waitForLine(t, stdout)

	assert.Len(t, heldFiles(t, dir), 1, "the live child's file was not read")

	require.NoError(t, cmd.Process.Kill())
	_ = cmd.Wait()
	assert.Empty(t, heldFiles(t, dir))
}

func TestAReheldFileHoldsTheNewDataAndIsGoneOnRelease(t *testing.T) {
	dir := t.TempDir()
	held, err := fs.HoldFile(fs.HoldFileParams{Path: filepath.Join(dir, "op-1.json"), Data: []byte(`{"locks":1}`)})
	require.NoError(t, err)

	reheld, err := fs.Rehold(fs.ReholdParams{Held: held, Data: []byte(`{"locks":2}`)})
	require.NoError(t, err)
	assert.True(t, reheld.Held)
	assert.Equal(t, [][]byte{[]byte(`{"locks":2}`)}, heldFiles(t, dir), "the old lock going did not take the new file with it")

	reheld.Release()
	assert.Empty(t, entries(t, dir))
}
