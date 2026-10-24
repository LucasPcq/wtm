package config

import (
	"fmt"
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
	return Project{StateDir: stateDir, GlobalPath: filepath.Join(t.TempDir(), "config.toml")}.Load()
}

// loadWithGlobal loads a minimal project with global written as the global
// config, and returns the global config read.
func loadWithGlobal(t *testing.T, global string) (project.Global, error) {
	t.Helper()
	globalPath := filepath.Join(t.TempDir(), "config.toml")
	writeFile(t, globalPath, global)
	_, loaded, err := Project{StateDir: stateWithConfig(t, minimalConfig), GlobalPath: globalPath}.Load()
	return loaded, err
}

// golden returns the content of testdata/name.
func golden(t *testing.T, name string) string {
	t.Helper()
	return readFile(t, filepath.Join("testdata", name))
}

// onDisk is the project whose state directory holds what s seeds, with no
// global config.
func onDisk(t *testing.T, s seed) Project {
	t.Helper()
	stateDir := t.TempDir()
	if s.Config != nil {
		data, err := render(savedProject{Config: *s.Config})
		require.NoError(t, err)
		if s.Unreadable {
			data = []byte("[worktrees\n")
		}
		if s.Schema != 0 {
			data = append([]byte(fmt.Sprintf("schema = %d\n", s.Schema)), data...)
		}
		writeFile(t, filepath.Join(stateDir, "config.toml"), string(data))
	}
	return Project{StateDir: stateDir, GlobalPath: filepath.Join(t.TempDir(), "config.toml")}
}
