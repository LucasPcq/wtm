package store_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/store"
)

// The few helpers the tests share. Everything a test file holds is a test.

const (
	envStateDir = "WTM_TEST_STORE_STATE_DIR"
	envJob      = "WTM_TEST_STORE_JOB"
	envRounds   = "WTM_TEST_STORE_ROUNDS"
)

func metaPath(stateDir, branch string) string {
	return filepath.Join(rules.WorktreeMetaDir(stateDir, branch), domain.MetaFileName)
}

func writeMeta(t *testing.T, stateDir, branch, content string) {
	t.Helper()
	writeFile(t, metaPath(stateDir, branch), content)
}

func writeRecord(t *testing.T, stateDir, branch string, meta domain.WorktreeMetadata) {
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

func addNamespace(job string) store.MetaUpdate {
	return store.MetaUpdate{Branch: "feat/a", Missing: store.MissingRefuse, Change: func(meta *domain.WorktreeMetadata) error {
		meta.Namespaces = append(meta.Namespaces, job)
		return nil
	}}
}

// TestHelperProcess is the body of the processes the cross-process test
// starts: it records its namespaces one update at a time, and is skipped
// otherwise.
func TestHelperProcess(t *testing.T) {
	stateDir := os.Getenv(envStateDir)
	if stateDir == "" {
		t.Skip("run by TestMetaUpdatesFromSeveralProcessesAllLand")
	}
	rounds, err := strconv.Atoi(os.Getenv(envRounds))
	require.NoError(t, err)
	meta := store.MetaDisk{StateDir: stateDir}
	for round := range rounds {
		require.NoError(t, meta.Update(t.Context(), addNamespace(os.Getenv(envJob)+strconv.Itoa(round))))
	}
}
