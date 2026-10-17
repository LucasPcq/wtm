package create_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/branch"
	"github.com/LucasPcq/wtm/next/internal/wtm/run"
	"github.com/LucasPcq/wtm/next/internal/wtm/worktree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestARepositoryWithoutConfigIsAPrecondition(t *testing.T) {
	w := newWorld(t, seed{})
	require.NoError(t, os.Remove(filepath.Join(w.stateDir, "config.toml")))

	_, err := w.deps.Observe(t.Context(), []string{branchName})

	failure, ok := errors.AsType[*wtm.Error](err)
	require.True(t, ok, err)
	assert.Equal(t, wtm.KindPrecondition, failure.Kind)
	assert.Equal(t, "create.config_not_found", failure.Code)
	assert.Equal(t, "no wtm config found — run `wtm init` first", failure.Message)
}

func TestTheFactsCountOnlyTheBranchesThatDifferFromOrigin(t *testing.T) {
	w := newWorld(t, seed{Behind: true})

	assert.Equal(t, map[string]branch.AheadBehind{"main": {Behind: 1}}, w.facts(t).Divergence)
}

func TestTheFactsLocateTheFolderOfEachBranchAsked(t *testing.T) {
	w := newWorld(t, seed{})
	require.NoError(t, os.MkdirAll(w.path("plain"), 0o755))

	facts := w.facts(t, "plain", " "+branchName+" ", "  ")

	assert.Equal(t, map[string]worktree.Location{w.path("plain"): {Path: w.path("plain"), Exists: true}}, facts.Taken)
}

func TestARunConfigThatDoesNotLoadIsAFactNotAFailure(t *testing.T) {
	w := newWorld(t, seed{Run: "[[job]]\nname = 3\n"})

	facts := w.facts(t)

	assert.Error(t, facts.RunErr)
	assert.Equal(t, run.Config{}, facts.Run)
}
