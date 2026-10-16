package shell

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/wtm/project"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAHookLogIsNamedAfterItsEscapedBranch(t *testing.T) {
	assert.Equal(t, filepath.Join("/state", "hooks", "on_create-feat%2Fx.log"), HookLogPath(HookLogParams{StateDir: "/state", Branch: "feat/x"}))
}

func TestNoStateDirMeansNoLog(t *testing.T) {
	assert.Empty(t, HookLogPath(HookLogParams{Branch: "feat/x"}))
}

func TestAHookLogHoldsOnlyTheLastRun(t *testing.T) {
	log := filepath.Join(t.TempDir(), "hooks", "on_create-x.log")

	require.NoError(t, RunHooks(t.Context(), HooksParams{Hooks: []project.Hook{{Cmd: "echo first run"}}, WorkDir: t.TempDir(), Log: log}))
	require.NoError(t, RunHooks(t.Context(), HooksParams{Hooks: []project.Hook{{Cmd: "echo second run"}, {Cmd: "printf unfinished"}}, WorkDir: t.TempDir(), Log: log}))

	content, err := os.ReadFile(log)
	require.NoError(t, err)
	assert.Equal(t, "second run\nunfinished\n", string(content))
}

func TestALogThatCannotBeOpenedNeverStopsTheHooks(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(blocker, nil, 0o644))
	out := &collected{}

	err := RunHooks(t.Context(), HooksParams{Hooks: []project.Hook{{Cmd: "echo ok"}}, WorkDir: t.TempDir(), Log: filepath.Join(blocker, "hooks", "x.log"), Output: out.add})

	require.NoError(t, err)
	assert.Equal(t, []string{"ok"}, out.all())
}

func TestNoHookToRunLeavesNoLog(t *testing.T) {
	log := filepath.Join(t.TempDir(), "hooks", "on_create-feat.log")

	require.NoError(t, RunHooks(t.Context(), HooksParams{WorkDir: t.TempDir(), Log: log}))
	require.NoError(t, RunHooks(t.Context(), HooksParams{Hooks: []project.Hook{{Cmd: "  "}}, WorkDir: t.TempDir(), Log: log}))

	assert.NoFileExists(t, log)
}
