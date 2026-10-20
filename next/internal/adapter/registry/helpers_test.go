package registry

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/stretchr/testify/require"
)

// registryIn returns a registry in a new directory, its clock fixed.
func registryIn(t *testing.T) Registry {
	t.Helper()
	return Registry{Path: filepath.Join(t.TempDir(), "wtm", "repos.json"), Now: func() time.Time { return time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC) }}
}

// initialized returns a repository whose common directory holds a wtm config.
func initialized(t *testing.T) domain.Repo {
	t.Helper()
	root := t.TempDir()
	commonDir := filepath.Join(root, ".git")
	require.NoError(t, os.MkdirAll(filepath.Join(commonDir, "wtm"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(commonDir, "wtm", "config.toml"), nil, 0o644))
	return domain.Repo{Root: root, CommonDir: commonDir}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}
