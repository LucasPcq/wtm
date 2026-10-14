package gitwt_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// untracked leaves a file the steps after an add would: a copied .env.
func untracked(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		return
	}
	require.NoError(t, os.WriteFile(filepath.Join(path, ".env"), []byte("PORT=3000\n"), 0o644))
}
