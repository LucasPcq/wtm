package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/stretchr/testify/require"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

// stateWithConfig returns a new state directory holding content as config.toml.
func stateWithConfig(t *testing.T, content string) string {
	t.Helper()
	stateDir := t.TempDir()
	writeFile(t, filepath.Join(stateDir, "config.toml"), content)
	return stateDir
}

// stateWithRun returns a new state directory holding content as run.toml.
func stateWithRun(t *testing.T, content string) string {
	t.Helper()
	stateDir := t.TempDir()
	writeFile(t, filepath.Join(stateDir, "run.toml"), content)
	return stateDir
}

// loadState loads the config of stateDir with no global config on disk.
func loadState(t *testing.T, stateDir string) (project.Config, project.Global, error) {
	t.Helper()
	return Load(LoadParams{StateDir: stateDir, GlobalPath: filepath.Join(t.TempDir(), "config.toml")})
}

// loadWithGlobal loads a minimal project with global written as the global
// config, and returns the global config read.
func loadWithGlobal(t *testing.T, global string) (project.Global, error) {
	t.Helper()
	globalPath := filepath.Join(t.TempDir(), "config.toml")
	writeFile(t, globalPath, global)
	_, loaded, err := Load(LoadParams{StateDir: stateWithConfig(t, minimalConfig), GlobalPath: globalPath})
	return loaded, err
}

// golden returns the content of testdata/name.
func golden(t *testing.T, name string) string {
	t.Helper()
	return readFile(t, filepath.Join("testdata", name))
}
