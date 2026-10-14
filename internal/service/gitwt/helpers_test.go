package gitwt_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func untracked(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		return
	}
	// A file the steps after an add would leave: a copied .env.
	require.NoError(t, os.WriteFile(filepath.Join(path, ".env"), []byte("PORT=3000\n"), 0o644))
}
