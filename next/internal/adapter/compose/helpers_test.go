package compose

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// repository is a new repository whose root holds testdata/docker-compose.yml as name.
func repository(t *testing.T, name string) Repository {
	t.Helper()
	dir := t.TempDir()
	data, err := os.ReadFile(filepath.Join("testdata", "docker-compose.yml"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), data, 0o644))
	return Repository{Dir: dir}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}
