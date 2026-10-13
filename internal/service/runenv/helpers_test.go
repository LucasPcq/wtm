package runenv_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// The few helpers the tests share. Everything a test file holds is a test.

// projectDir is the main checkout, named after the project.
func projectDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), project)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "infra"), 0o755))
	return dir
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}
