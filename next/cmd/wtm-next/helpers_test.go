package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/api"
	"github.com/stretchr/testify/require"
)

// repository returns the main worktree of a new repository configured for wtm, its worktrees under ../.trees.
func repository(t *testing.T) string {
	t.Helper()
	main := filepath.Join(gittest.TempDir(t), "main")
	gittest.Build(t, gittest.FakeParams{Main: main, Branch: "main"})
	stateDir := filepath.Join(main, ".git", "wtm")
	require.NoError(t, os.MkdirAll(stateDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, "config.toml"), []byte("[worktrees]\nbase_path = \"../.trees\"\n"), 0o644))
	return main
}

// command returns the command of the catalogue called name.
func command(t *testing.T, name string) api.Command {
	t.Helper()
	found, ok := catalogue().Find(name)
	require.True(t, ok, name)
	return found
}
