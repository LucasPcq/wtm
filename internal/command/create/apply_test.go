package create_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/command/create"
	"github.com/LucasPcq/wtm/internal/dispatch"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

func TestEachBranchGetsItsWorktreeEnvAndRecord(t *testing.T) {
	w := newWorld(t, seed{})

	result := w.apply(t, create.Request{Branches: []string{branch, existing, remote}, From: "main"})

	assert.Equal(t, map[string]kernel.Status{branch: kernel.StatusDone, existing: kernel.StatusDone, remote: kernel.StatusDone}, statuses(result.Result.Outcome.Items))
	for _, name := range []string{branch, existing, remote} {
		assert.Equal(t, name, gittest.Output(t, w.path(name), "branch", "--show-current"))
		assert.Equal(t, mainEnv, readFile(t, filepath.Join(w.path(name), ".env")))
		recorded, err := w.disk.Meta.Load(name)
		require.NoError(t, err)
		assert.Equal(t, "main", recorded.SourceBranch)
	}
	assert.Contains(t, gittest.Output(t, w.main, "config", "branch."+remote+".merge"), remote)
	assert.True(t, result.Result.Outcome.Items[1].Detail.ExistingBranch)
	assert.Equal(t, "origin/"+remote, result.Result.Outcome.Items[2].Detail.Upstream)
}

func TestOneBranchThatFailsLeavesTheOthersCreated(t *testing.T) {
	w := newWorld(t, seed{Config: withHook(`test "$(git branch --show-current)" != feat/y`)})

	result := w.apply(t, create.Request{Branches: []string{branch, other}})

	items := result.Result.Outcome.Items
	assert.Equal(t, map[string]kernel.Status{branch: kernel.StatusDone, other: kernel.StatusFailed}, statuses(items))
	assert.Equal(t, kernel.ReasonIncomplete, items[1].Reason)
	assert.DirExists(t, w.path(other))
	assert.NotNil(t, result.Result.Outcome.Failure())
}

func TestABranchAlreadyOpenIsUnchangedAndNothingIsWritten(t *testing.T) {
	w := newWorld(t, seed{Linked: []string{"linked"}})

	result := w.apply(t, create.Request{Branches: []string{"linked"}, IfNotExists: true})

	item := result.Result.Outcome.Items[0]
	assert.Equal(t, kernel.StatusDone, item.Status)
	assert.Equal(t, kernel.ReasonUnchanged, item.Reason)
	assert.Equal(t, linkedPath(w.root, "linked"), item.Detail.Path)
	assert.Equal(t, []domain.EventType{domain.EventOperationStarted, domain.EventOperationFinished}, w.bench.Bus.Types())
}

func TestTheSourceIsFastForwardedBeforeTheWorktreeStartsFromIt(t *testing.T) {
	w := newWorld(t, seed{Behind: true, NoEnv: true})

	result := w.apply(t, create.Request{Branches: []string{branch}, FastForward: true})

	assert.Empty(t, result.Result.Outcome.Warnings)
	upstream := gittest.Output(t, w.main, "rev-parse", "origin/main")
	assert.Equal(t, upstream, gittest.Output(t, w.main, "rev-parse", "main"))
	assert.Equal(t, upstream, gittest.Output(t, w.path(branch), "rev-parse", "HEAD"))
}

func TestASourceWithUncommittedChangesIsLeftWhereItIsAndSaysSo(t *testing.T) {
	w := newWorld(t, seed{Behind: true})

	result := w.apply(t, create.Request{Branches: []string{branch}, FastForward: true})

	assert.Equal(t, []kernel.Warning{{Code: create.CodeFastForwardDirty, Params: kernel.Params{create.ParamBranch: "main"}}}, result.Result.Outcome.Warnings)
	assert.Equal(t, kernel.StatusDone, result.Result.Outcome.Items[0].Status)
	assert.NotEqual(t, gittest.Output(t, w.main, "rev-parse", "origin/main"), gittest.Output(t, w.path(branch), "rev-parse", "HEAD"))
}

func TestTheAnswersAskedToBeRememberedAreKeptOnceABranchIsCreated(t *testing.T) {
	w := newWorld(t, seed{Run: jobs})

	w.apply(t, create.Request{Branches: []string{branch}, EnvFrom: domain.EnvStrategyExample, Isolation: domain.IsolationVerbatim, Remember: []string{create.PathEnvFrom, create.PathIsolation}})

	config := readFile(t, filepath.Join(w.stateDir, domain.ConfigFileName))
	assert.Contains(t, config, `env_strategy = "example"`)
	assert.Contains(t, config, `isolation = "verbatim"`)
}

func TestNothingIsRememberedWhenNoBranchWasCreated(t *testing.T) {
	w := newWorld(t, seed{Linked: []string{"linked"}})

	w.apply(t, create.Request{Branches: []string{"linked"}, IfNotExists: true, EnvFrom: domain.EnvStrategyExample, Remember: []string{create.PathEnvFrom}})

	assert.NotContains(t, readFile(t, filepath.Join(w.stateDir, domain.ConfigFileName)), "env_strategy")
}

func TestAnInterruptedRunSkipsTheBranchesItNeverReached(t *testing.T) {
	w := newWorld(t, seed{Config: withHook("touch started; sleep 5")})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go func() {
		<-appears(filepath.Join(w.path(branch), "started"))
		cancel()
	}()

	result := w.run(ctx, runParams{Request: create.Request{Branches: []string{branch, other}}})

	require.NoError(t, result.Err)
	items := result.Result.Outcome.Items
	assert.Equal(t, kernel.StatusCancelled, items[0].Status)
	assert.Equal(t, kernel.StatusSkipped, items[1].Status)
	assert.Equal(t, kernel.ReasonInterrupted, items[1].Reason)
	assert.NoDirExists(t, w.path(other))
}

func TestARunWhoseFactsChangedSinceItsPreviewIsRefused(t *testing.T) {
	w := newWorld(t, seed{})
	preview := w.run(t.Context(), runParams{Request: create.Request{Branches: []string{branch}}, Options: dispatch.Options{DryRun: true}})
	require.NoError(t, preview.Err)
	gittest.CreateBranch(t, w.main, "moved")

	stale := w.run(t.Context(), runParams{Request: create.Request{Branches: []string{branch}}, Options: dispatch.Options{ExpectFingerprint: preview.Result.Fingerprint}})

	var failure kernel.Error
	require.ErrorAs(t, stale.Err, &failure)
	assert.Equal(t, kernel.CodePlanStale, failure.Base().Code)
	_, err := os.Stat(w.path(branch))
	assert.ErrorIs(t, err, os.ErrNotExist)
}
