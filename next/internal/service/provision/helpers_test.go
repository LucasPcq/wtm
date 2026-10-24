package provision_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
	"github.com/stretchr/testify/require"
)

func codes(warnings []domain.Warning) []string {
	found := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		found = append(found, warning.Code)
	}
	return found
}

func gitState(t *testing.T, w world) gittest.State {
	t.Helper()
	return gittest.StateOf(t, w.git)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(content)
}

func gitStateNames(t *testing.T, w world) []string {
	t.Helper()
	return gittest.Names(gitState(t, w).Branches.Local)
}

func fastForwardEach(snapshot worktree.Snapshot, subjects ...string) provision.FastForwardEachParams {
	return provision.FastForwardEachParams{Command: "create", Subjects: subjects, Snapshot: snapshot, Fallback: "it starts from your local branch"}
}
