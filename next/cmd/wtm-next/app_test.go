package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/app"
	"github.com/LucasPcq/wtm/next/internal/app/create"
	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheAppIsWiredOntoTheRealAdapters(t *testing.T) {
	root := gittest.TempDir(t)
	main := filepath.Join(root, "main")
	gittest.Build(t, gittest.FakeParams{Main: main, Branch: "main"})
	stateDir := filepath.Join(main, ".git", "wtm")
	require.NoError(t, os.MkdirAll(stateDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, "config.toml"), []byte("[worktrees]\nbase_path = \"../.trees\"\n"), 0o644))
	t.Setenv("HOME", t.TempDir())

	outcome, err := newApp(appParams{MainPath: main, StateDir: stateDir}).Create(t.Context(), app.CreateParams{Request: create.Request{Branches: []string{"feat/x"}}, Report: func(wtm.Event) {}})

	require.NoError(t, err)
	require.NotNil(t, outcome.Result)
	assert.Equal(t, wtm.StatusDone, outcome.Result.Items[0].Status, outcome.Result.Items[0].Err)
	assert.NotEmpty(t, outcome.Result.Items[0].Detail.Metadata.CreatedAt)
	assert.Len(t, outcome.Op, 16)
	assert.DirExists(t, filepath.Join(root, ".trees", "feat-x"))
}
