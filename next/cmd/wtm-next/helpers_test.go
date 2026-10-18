package main

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/api"
	"github.com/stretchr/testify/require"
)

// configured returns the main worktree of a new repository configured for wtm, its worktrees under ../.trees.
func configured(t *testing.T) string {
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

// registeredCommands reads main.go for the commands it registers: each command/<name>/cli it imports, which Go refuses unused.
func registeredCommands(t *testing.T) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, parser.ImportsOnly)
	require.NoError(t, err)
	var names []string
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		require.NoError(t, err)
		name, ok := strings.CutPrefix(path, commandPrefix)
		if name, isCLI := strings.CutSuffix(name, "/cli"); ok && isCLI {
			names = append(names, name)
		}
	}
	return names
}
