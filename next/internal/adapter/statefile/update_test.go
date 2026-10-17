package statefile

import (
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnUpdateLosesNoConcurrentUpdate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "counter")
	const writers = 32

	var wg sync.WaitGroup
	for range writers {
		wg.Go(func() { increment(t, path) })
	}
	wg.Wait()

	assert.Equal(t, writers, counter(t, path))
}

func TestAnUpdateLosesNoUpdateAcrossProcesses(t *testing.T) {
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

func TestHelperProcess(t *testing.T) {
	path := os.Getenv(envCounterPath)
	if path == "" {
		t.Skip("run by TestAnUpdateLosesNoUpdateAcrossProcesses")
	}
	rounds, err := strconv.Atoi(os.Getenv(envCounterRounds))
	require.NoError(t, err)
	for range rounds {
		increment(t, path)
	}
}

func TestMutateSeesNilForAMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent")
	seen := []byte("not called")

	_, err := Update(t.Context(), UpdateParams{Path: path, Mutate: func(current []byte) (Change, error) {
		seen = current
		return Keep{}, nil
	}})

	require.NoError(t, err)
	assert.Nil(t, seen)
}

func TestKeepLeavesAMissingFileMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent")

	_, err := Update(t.Context(), UpdateParams{Path: path, Mutate: func([]byte) (Change, error) { return Keep{}, nil }})

	require.NoError(t, err)
	assert.NoFileExists(t, path)
}

func TestAMutateDecidingNothingIsAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")

	_, err := Update(t.Context(), UpdateParams{Path: path, Mutate: func([]byte) (Change, error) { return nil, nil }})

	assert.Error(t, err)
	assert.NoFileExists(t, path)
}

func TestTheLockSitsBesideTheFileByDefault(t *testing.T) {
	dir := t.TempDir()

	set(t, setParams{Path: filepath.Join(dir, "config.toml"), Content: "x"})

	assert.ElementsMatch(t, []string{"config.toml", "config.toml.lock"}, entries(t, dir))
}

func TestAFileInTheUsersTreeLeavesNoLockFileThere(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")

	_, err := Update(t.Context(), UpdateParams{
		Path:     path,
		LockPath: ScratchLockPath(path),
		Mutate:   func([]byte) (Change, error) { return Write{Data: []byte("PORT=3000\n")}, nil },
	})

	require.NoError(t, err)
	assert.Equal(t, []string{".env"}, entries(t, dir))
}

func TestAnUnavailableLockStillWritesAndSaysSo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.Mkdir(path+".lock", 0o755))

	written := increment(t, path)

	assert.True(t, written.Unlocked)
	assert.Equal(t, 1, counter(t, path))
}

func TestATakenLockSaysSo(t *testing.T) {
	assert.False(t, increment(t, filepath.Join(t.TempDir(), "file")).Unlocked)
}

func TestAnUpdateFailsWhenItsDirectoryCannotBeMade(t *testing.T) {
	notADir := filepath.Join(t.TempDir(), "file")
	writeFixture(t, notADir, "")

	_, err := Update(t.Context(), UpdateParams{Path: filepath.Join(notADir, "state.json"), Mutate: func([]byte) (Change, error) {
		return Write{Data: []byte("{}")}, nil
	}})

	assert.Error(t, err)
}

func TestAnUpdateFailsOnSomethingItCannotRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dir")
	require.NoError(t, os.Mkdir(path, 0o755))
	called := false

	_, err := Update(t.Context(), UpdateParams{Path: path, Mutate: func([]byte) (Change, error) {
		called = true
		return Keep{}, nil
	}})

	assert.Error(t, err)
	assert.False(t, called)
}

func TestAFailedMutateIsNeverSeenHalfWritten(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file")
	writeFixture(t, path, "kept")
	refused := errors.New("refused")

	_, err := Update(t.Context(), UpdateParams{Path: path, Mutate: func([]byte) (Change, error) {
		return Write{Data: []byte("lost")}, refused
	}})

	assert.ErrorIs(t, err, refused)
	assert.Equal(t, "kept", contentOf(t, path))
	assert.ElementsMatch(t, []string{"file", "file.lock"}, entries(t, dir))
}

func TestAWriteFailingMidwayLeavesTheOriginalAndNoTemporaryFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	set(t, setParams{Path: path, Content: "original"})
	cmd := exec.Command(os.Args[0], "-test.run=^TestAProcessWhoseFilesCannotGrow$")
	cmd.Env = append(os.Environ(), envGrowPath+"="+path)

	out, err := cmd.CombinedOutput()

	require.NoError(t, err, string(out))
	assert.Equal(t, "original", contentOf(t, path))
	assert.ElementsMatch(t, []string{"state.json", "state.json.lock"}, entries(t, dir))
}

func TestAProcessWhoseFilesCannotGrow(t *testing.T) {
	path := os.Getenv(envGrowPath)
	if path == "" {
		t.Skip("run by TestAWriteFailingMidwayLeavesTheOriginalAndNoTemporaryFile")
	}
	signal.Ignore(syscall.SIGXFSZ)
	var limit syscall.Rlimit
	require.NoError(t, syscall.Getrlimit(syscall.RLIMIT_FSIZE, &limit))
	limit.Cur = 4
	require.NoError(t, syscall.Setrlimit(syscall.RLIMIT_FSIZE, &limit))

	_, err := Update(t.Context(), UpdateParams{Path: path, Mutate: func([]byte) (Change, error) {
		return Write{Data: []byte("a write far longer than four bytes")}, nil
	}})

	require.ErrorIs(t, err, syscall.EFBIG)
}

func TestAnUpdateSaysWhetherItChangedTheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	write := func(content string) domain.Written {
		written, err := Update(t.Context(), UpdateParams{Path: path, Mutate: func([]byte) (Change, error) { return Write{Data: []byte(content)}, nil }})
		require.NoError(t, err)
		return written
	}

	assert.True(t, write("a").Changed)
	assert.False(t, write("a").Changed)
	assert.True(t, write("b").Changed)
}
