package initialize_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/stretchr/testify/require"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

// configure writes content as config.toml.
func (w world) configure(t *testing.T, content string) {
	t.Helper()
	writeFile(t, filepath.Join(w.stateDir, "config.toml"), content)
}

// saved is config.toml as it now stands on disk.
func (w world) saved(t *testing.T) project.Config {
	t.Helper()
	saved, err := w.usecase.Settings.Store.Saved()
	require.NoError(t, err)
	return saved.Config
}

func (w world) configText(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(w.stateDir, "config.toml"))
	require.NoError(t, err)
	return string(data)
}

func failureOf(t *testing.T, err error) *domain.Error {
	t.Helper()
	failure, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok, err)
	return failure
}

func codes(followUps []domain.FollowUp) []string {
	found := make([]string, 0, len(followUps))
	for _, followUp := range followUps {
		found = append(found, followUp.Code)
	}
	return found
}
