package detect

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// repository is a directory holding files, by their slash-separated path.
func repository(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		at := filepath.Join(dir, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(at), 0o755))
		require.NoError(t, os.WriteFile(at, []byte(content), 0o644))
	}
	return dir
}
