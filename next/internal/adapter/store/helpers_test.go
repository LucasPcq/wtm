package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/dotenv"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/stretchr/testify/require"
)

// metaPath returns where the record of branch lives in stateDir.
func metaPath(t *testing.T, stateDir, branch string) string {
	t.Helper()
	dir, ok := worktree.MetaDir(worktree.MetaDirParams{StateDir: stateDir, Branch: branch})
	require.True(t, ok)
	return filepath.Join(dir, "meta.json")
}

// writeMeta writes content as the record of branch, byte for byte.
func writeMeta(t *testing.T, stateDir, branch, content string) {
	t.Helper()
	writeFile(t, metaPath(t, stateDir, branch), content)
}

// writeRecord writes meta as the record of branch, as JSON.
func writeRecord(t *testing.T, stateDir, branch string, meta worktree.Meta) {
	t.Helper()
	data, err := json.MarshalIndent(meta, "", "  ")
	require.NoError(t, err)
	writeMeta(t, stateDir, branch, string(data))
}

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

// addNamespace returns an update that adds job to the namespaces of feat/a.
func addNamespace(job string) worktree.MetaUpdate {
	return worktree.MetaUpdate{Branch: "feat/a", Change: func(meta *worktree.Meta) error {
		meta.Namespaces = append(meta.Namespaces, job)
		return nil
	}}
}

// errOf returns the error of a write, to assert on it in one line.
func errOf(_ domain.Written, err error) error {
	return err
}

// upsert returns an edit of a .env file's lines that sets key to value.
func upsert(key, value string) func([]dotenv.Line) []dotenv.Line {
	return func(lines []dotenv.Line) []dotenv.Line {
		updated, _ := dotenv.Set(dotenv.SetParams{Lines: lines, Key: key, Value: value})
		return updated
	}
}
