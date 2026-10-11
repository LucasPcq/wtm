package fs_test

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/infra/fs"
)

// The few helpers the tests share. Everything a test file holds is a test.

const (
	envCounterPath   = "WTM_TEST_COUNTER_PATH"
	envCounterRounds = "WTM_TEST_COUNTER_ROUNDS"
)

// increment adds one to the counter file at path, through UpdateFile.
func increment(t testing.TB, path string) fs.Lock {
	t.Helper()
	lock, err := fs.UpdateFile(t.Context(), fs.UpdateFileParams{
		Path: path,
		Mutate: func(current []byte) (fs.Change, error) {
			count := 0
			if current != nil {
				parsed, err := strconv.Atoi(string(current))
				if err != nil {
					return nil, err
				}
				count = parsed
			}
			return fs.Write{Data: []byte(strconv.Itoa(count + 1))}, nil
		},
	})
	require.NoError(t, err)
	return lock
}

func counter(t *testing.T, path string) int {
	t.Helper()
	count, err := strconv.Atoi(read(t, path))
	require.NoError(t, err)
	return count
}

// set writes content through UpdateFile, creating the file with perm.
func set(t *testing.T, path, content string, perm os.FileMode) {
	t.Helper()
	_, err := fs.UpdateFile(t.Context(), fs.UpdateFileParams{Path: path, Perm: perm, Mutate: func([]byte) (fs.Change, error) {
		return fs.Write{Data: []byte(content)}, nil
	}})
	require.NoError(t, err)
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

func modeOf(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	require.NoError(t, err)
	return info.Mode().Perm()
}

// entries names what a directory holds.
func entries(t *testing.T, dir string) []string {
	t.Helper()
	listed, err := os.ReadDir(dir)
	require.NoError(t, err)
	names := make([]string, 0, len(listed))
	for _, entry := range listed {
		names = append(names, entry.Name())
	}
	return names
}

func writeFixture(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

// TestHelperProcess is the body of the processes the cross-process test
// starts: it increments the counter it is pointed at, and is skipped otherwise.
func TestHelperProcess(t *testing.T) {
	path := os.Getenv(envCounterPath)
	if path == "" {
		t.Skip("run by TestUpdateFileLosesNoUpdateAcrossProcesses")
	}
	rounds, err := strconv.Atoi(os.Getenv(envCounterRounds))
	require.NoError(t, err)
	for range rounds {
		increment(t, path)
	}
}
