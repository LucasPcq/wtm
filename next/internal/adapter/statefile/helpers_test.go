package statefile

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"

	"github.com/stretchr/testify/require"
)

// The variables that turn the test binary into a helper process: the counter
// file it increments, how many times, and the file it grows.
const (
	envCounterPath   = "WTM_NEXT_TEST_COUNTER_PATH"
	envCounterRounds = "WTM_NEXT_TEST_COUNTER_ROUNDS"
	envGrowPath      = "WTM_NEXT_TEST_GROW_PATH"
)

// increment adds one to the counter file at path through Update, the way two
// racing processes do in the tests.
func increment(t testing.TB, path string) domain.Written {
	t.Helper()
	written, err := Update(t.Context(), UpdateParams{
		Path: path,
		Mutate: func(current []byte) (Change, error) {
			count := 0
			if current != nil {
				parsed, err := strconv.Atoi(string(current))
				if err != nil {
					return nil, err
				}
				count = parsed
			}
			return Write{Data: []byte(strconv.Itoa(count + 1))}, nil
		},
	})
	require.NoError(t, err)
	return written
}

// counter returns the number in the counter file at path.
func counter(t *testing.T, path string) int {
	t.Helper()
	count, err := strconv.Atoi(contentOf(t, path))
	require.NoError(t, err)
	return count
}

// setParams is the file set writes, its content and its permissions.
type setParams struct {
	Path    string
	Content string
	Perm    os.FileMode
}

// set writes params.Content to params.Path through Update.
func set(t *testing.T, params setParams) {
	t.Helper()
	_, err := Update(t.Context(), UpdateParams{Path: params.Path, Perm: params.Perm, Mutate: func([]byte) (Change, error) {
		return Write{Data: []byte(params.Content)}, nil
	}})
	require.NoError(t, err)
}

func contentOf(t *testing.T, path string) string {
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

// entries returns the names in dir, to check a write left no temporary file.
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
