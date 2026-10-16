package statefile

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	envCounterPath   = "WTM_NEXT_TEST_COUNTER_PATH"
	envCounterRounds = "WTM_NEXT_TEST_COUNTER_ROUNDS"
)

func increment(t testing.TB, path string) Lock {
	t.Helper()
	lock, err := Update(t.Context(), UpdateParams{
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
	return lock
}

func counter(t *testing.T, path string) int {
	t.Helper()
	count, err := strconv.Atoi(contentOf(t, path))
	require.NoError(t, err)
	return count
}

type setParams struct {
	Path    string
	Content string
	Perm    os.FileMode
}

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
