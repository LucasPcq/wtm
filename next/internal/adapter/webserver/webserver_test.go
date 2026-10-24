package webserver

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOneProcessAtATimeHoldsTheLock(t *testing.T) {
	files := Files{Dir: t.TempDir()}

	release, err := files.Lock()
	require.NoError(t, err)
	_, held := files.Lock()
	release()
	again, free := files.Lock()

	assert.ErrorIs(t, held, ErrRunning)
	require.NoError(t, free)
	again()
}

func TestTheRecordIsReadBackAndKeptFromOtherUsers(t *testing.T) {
	files := Files{Dir: t.TempDir()}
	record := Record{PID: 42, Port: 4777, Version: "0.31.0", Token: "secret"}

	require.NoError(t, files.Write(t.Context(), record))
	got, err := files.Read()

	require.NoError(t, err)
	assert.Equal(t, record, got)
	info, err := os.Stat(filepath.Join(files.Dir, recordName))
	require.NoError(t, err)
	assert.Equal(t, recordPerm, info.Mode().Perm())
}

func TestARecordWiderThanItsUserIsNarrowedOnWrite(t *testing.T) {
	files := Files{Dir: t.TempDir()}
	require.NoError(t, os.WriteFile(filepath.Join(files.Dir, recordName), []byte("{}"), 0o644))

	require.NoError(t, files.Write(t.Context(), Record{Token: "secret"}))

	info, err := os.Stat(filepath.Join(files.Dir, recordName))
	require.NoError(t, err)
	assert.Equal(t, recordPerm, info.Mode().Perm())
}

func TestNoRecordIsNotExist(t *testing.T) {
	_, err := Files{Dir: t.TempDir()}.Read()

	assert.ErrorIs(t, err, fs.ErrNotExist)
}
