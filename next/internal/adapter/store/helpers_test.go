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

func metaPath(t *testing.T, stateDir, branch string) string {
	t.Helper()
	dir, ok := worktree.MetaDir(worktree.MetaDirParams{StateDir: stateDir, Branch: branch})
	require.True(t, ok)
	return filepath.Join(dir, "meta.json")
}

func writeMeta(t *testing.T, stateDir, branch, content string) {
	t.Helper()
	writeFile(t, metaPath(t, stateDir, branch), content)
}

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

func addNamespace(job string) worktree.MetaUpdate {
	return worktree.MetaUpdate{Branch: "feat/a", Change: func(meta *worktree.Meta) error {
		meta.Namespaces = append(meta.Namespaces, job)
		return nil
	}}
}

func errOf(_ domain.Written, err error) error {
	return err
}

func upsert(key, value string) func([]dotenv.Line) []dotenv.Line {
	return func(lines []dotenv.Line) []dotenv.Line {
		updated, _ := dotenv.Set(dotenv.SetParams{Lines: lines, Key: key, Value: value})
		return updated
	}
}
