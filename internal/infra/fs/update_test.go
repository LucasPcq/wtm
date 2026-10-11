package fs_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/infra/fs"
)

// — Concurrency: no update is lost —

func TestUpdateFileLosesNoConcurrentUpdate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "counter")
	const writers = 32

	var wg sync.WaitGroup
	for range writers {
		wg.Go(func() { increment(t, path) })
	}
	wg.Wait()

	assert.Equal(t, writers, counter(t, path))
}

func TestUpdateFileLosesNoUpdateAcrossProcesses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "counter")
	const processes, rounds = 4, 25

	var wg sync.WaitGroup
	for range processes {
		wg.Go(func() {
			cmd := exec.Command(os.Args[0], "-test.run=^TestHelperProcess$")
			cmd.Env = append(os.Environ(), envCounterPath+"="+path, envCounterRounds+"="+strconv.Itoa(rounds))
			out, err := cmd.CombinedOutput()
			assert.NoError(t, err, string(out))
		})
	}
	wg.Wait()

	assert.Equal(t, processes*rounds, counter(t, path))
}

// — What Mutate decides —

func TestMutateSeesNilForAMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent")
	seen := []byte("not called")
	_, err := fs.UpdateFile(t.Context(), fs.UpdateFileParams{Path: path, Mutate: func(current []byte) (fs.Change, error) {
		seen = current
		return fs.Keep{}, nil
	}})
	require.NoError(t, err)
	assert.Nil(t, seen)
}

func TestWriteCreatesWithItsPermAndAnExistingFileKeepsItsMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	set(t, path, "a", 0o600)
	assert.Equal(t, os.FileMode(0o600), modeOf(t, path))

	set(t, path, "b", 0o644)
	assert.Equal(t, os.FileMode(0o600), modeOf(t, path))
	assert.Equal(t, "b", read(t, path))
}

func TestWriteWithoutPermCreatesAReadableFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	set(t, path, "a", 0)
	assert.Equal(t, os.FileMode(0o644), modeOf(t, path))
}

func TestWritingTheSameContentLeavesTheFileAlone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	writeFixture(t, path, "same")
	before, err := os.Stat(path)
	require.NoError(t, err)

	set(t, path, "same", 0)

	after, err := os.Stat(path)
	require.NoError(t, err)
	assert.True(t, os.SameFile(before, after), "an unchanged file was replaced")
}

func TestKeepLeavesAMissingFileMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent")
	_, err := fs.UpdateFile(t.Context(), fs.UpdateFileParams{Path: path, Mutate: func([]byte) (fs.Change, error) {
		return fs.Keep{}, nil
	}})
	require.NoError(t, err)
	assert.NoFileExists(t, path)
}

func TestRemoveDeletesTheFileAndIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	writeFixture(t, path, "x")
	for range 2 {
		_, err := fs.UpdateFile(t.Context(), fs.UpdateFileParams{Path: path, Mutate: func([]byte) (fs.Change, error) {
			return fs.Remove{}, nil
		}})
		require.NoError(t, err)
	}
	assert.NoFileExists(t, path)
}

func TestAFailedMutateWritesNothingAndLeavesNoTemporaryFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file")
	writeFixture(t, path, "kept")
	refused := errors.New("refused")

	_, err := fs.UpdateFile(t.Context(), fs.UpdateFileParams{Path: path, Mutate: func([]byte) (fs.Change, error) {
		return fs.Write{Data: []byte("lost")}, refused
	}})

	assert.ErrorIs(t, err, refused)
	assert.Equal(t, "kept", read(t, path))
	assert.ElementsMatch(t, []string{"file", "file.lock"}, entries(t, dir))
}

func TestAMutateDecidingNothingIsAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	_, err := fs.UpdateFile(t.Context(), fs.UpdateFileParams{Path: path, Mutate: func([]byte) (fs.Change, error) {
		return nil, nil
	}})
	assert.Error(t, err)
	assert.NoFileExists(t, path)
}

// — Where the lock lives —

func TestTheLockSitsBesideTheFileByDefault(t *testing.T) {
	dir := t.TempDir()
	set(t, filepath.Join(dir, "config.toml"), "x", 0)
	assert.ElementsMatch(t, []string{"config.toml", "config.toml.lock"}, entries(t, dir))
}

func TestAFileInTheUsersTreeLeavesNoLockFileThere(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	_, err := fs.UpdateFile(t.Context(), fs.UpdateFileParams{
		Path:     path,
		LockPath: fs.ScratchLockPath(path),
		Mutate:   func([]byte) (fs.Change, error) { return fs.Write{Data: []byte("PORT=3000\n")}, nil },
	})
	require.NoError(t, err)
	assert.Equal(t, []string{".env"}, entries(t, dir))
}

func TestAnUnavailableLockStillWritesAndSaysSo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.Mkdir(path+".lock", 0o755))

	lock := increment(t, path)

	assert.False(t, lock.Held)
	assert.Equal(t, 1, counter(t, path))
}

func TestATakenLockSaysSo(t *testing.T) {
	lock := increment(t, filepath.Join(t.TempDir(), "file"))
	assert.True(t, lock.Held)
}

// — When the disk refuses —

func TestUpdateFileFailsWhenItsDirectoryCannotBeMade(t *testing.T) {
	notADir := filepath.Join(t.TempDir(), "file")
	writeFixture(t, notADir, "")
	_, err := fs.UpdateFile(t.Context(), fs.UpdateFileParams{Path: filepath.Join(notADir, "state.json"), Mutate: func([]byte) (fs.Change, error) {
		return fs.Write{Data: []byte("{}")}, nil
	}})
	assert.Error(t, err)
}

func TestUpdateFileFailsOnSomethingItCannotRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dir")
	require.NoError(t, os.Mkdir(path, 0o755))
	called := false
	_, err := fs.UpdateFile(t.Context(), fs.UpdateFileParams{Path: path, Mutate: func([]byte) (fs.Change, error) {
		called = true
		return fs.Keep{}, nil
	}})
	assert.Error(t, err)
	assert.False(t, called)
}
