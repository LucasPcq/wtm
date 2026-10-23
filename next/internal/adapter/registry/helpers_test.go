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

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

func keepAll(domain.Repo) bool { return true }
