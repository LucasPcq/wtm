package runinit_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/store"
	"github.com/LucasPcq/wtm/next/internal/command/runinit"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
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

// record numbers feat/billing 2.
func (w *world) record(t *testing.T, meta store.Meta) {
	t.Helper()
	_, err := meta.Update(t.Context(), worktree.MetaUpdate{Branch: billing, Change: func(stored *worktree.Meta) error {
		stored.Ordinal, stored.SourceBranch = 2, "main"
		return nil
	}})
	require.NoError(t, err)
}

// apply runs req to its end, every file written.
func (w *world) apply(t *testing.T, req runinit.Request) runinit.Outcome {
	t.Helper()
	outcome, err := w.usecase.Run(t.Context(), req, domain.RunOptions{})
	require.NoError(t, err)
	require.NotNil(t, outcome.Result)
	require.Nil(t, outcome.Result.Failure(), "%+v", outcome.Result.Items)
	return outcome
}

func (w *world) runToml(t *testing.T) string {
	t.Helper()
	return readFile(t, filepath.Join(w.stateDir, "run.toml"))
}
