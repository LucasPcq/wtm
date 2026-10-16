package provision_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/command/provision"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/kernel/kerneltest"
	"github.com/LucasPcq/wtm/internal/testutil/storetest"
)

func TestTheSagaLeavesNothingWhereverItFails(t *testing.T) {
	for name, params := range map[string]func(world) provision.UnitParams{
		"a new branch":       world.params,
		"an existing branch": world.existing,
	} {
		t.Run(name, func(t *testing.T) {
			w := onDisk(t)

			kerneltest.CheckSaga(t, kerneltest.SagaParams[provision.Detail]{
				Saga:     func() kernel.Saga[provision.Detail] { return provision.Unit(params(w)).Saga },
				Snapshot: snapshot(t, w),
			})
		})
	}
}

func TestAWorktreeIsAddedWithItsEnvAndItsRecord(t *testing.T) {
	w := onDisk(t)

	item := run(t.Context(), runParams{Unit: provision.Unit(w.params())})

	require.Equal(t, kernel.StatusDone, item.Status, item.Error)
	metadata := domain.WorktreeMetadata{SourceBranch: "main", CreatedAt: "2026-10-16T12:00:00Z", EnvStrategy: domain.EnvStrategyMain, Isolation: domain.IsolationIsolated}
	assert.Equal(t, provision.Detail{Path: w.path(branch), Metadata: metadata}, item.Detail)
	assert.Equal(t, mainEnv, readFile(t, filepath.Join(w.path(branch), ".env")))
	recorded, err := w.meta.Load(branch)
	require.NoError(t, err)
	assert.Equal(t, metadata, recorded)
}

func TestAnExistingBranchCarriesWhereItStandsAgainstOrigin(t *testing.T) {
	w := onDisk(t)
	params := w.existing()
	params.Worktree.Origin = provision.Origin{ExistingBranch: true, OriginState: "behind", OriginBehind: 2}

	item := run(t.Context(), runParams{Unit: provision.Unit(params)})

	require.Equal(t, kernel.StatusDone, item.Status, item.Error)
	assert.Equal(t, params.Worktree.Origin, item.Detail.Origin)
}

func TestEachEnvThatCouldNotBeCopiedIsAWarningOfTheItem(t *testing.T) {
	w := onDisk(t)
	require.NoError(t, os.Remove(filepath.Join(w.main, ".env")))

	item := run(t.Context(), runParams{Unit: provision.Unit(w.params())})

	require.Equal(t, kernel.StatusDone, item.Status, item.Error)
	assert.Equal(t, []kernel.Warning{{Code: provision.CodeEnvSourceMissing, Params: kernel.Params{provision.ParamTarget: ".env", provision.ParamDir: w.main}}}, item.Warnings)
}

func TestAnUndoThatFailsNamesWhatIsLeftAndTheWayOut(t *testing.T) {
	w, _ := inMemory(t, seeds{
		GitFail: map[string]error{"RemoveWorktree": errors.New("worktree is locked")},
		Store:   storetest.FakeParams{Fail: map[string]error{"Update": errors.New("disk full")}},
	})

	item := run(t.Context(), runParams{Unit: provision.Unit(w.params())})

	require.Equal(t, kernel.StatusFailed, item.Status)
	assert.Equal(t, kernel.CodeUndoFailed, item.Error.Base().Code)
	followUp := item.Error.Base().FollowUp
	require.NotNil(t, followUp)
	assert.Equal(t, domain.CmdClean, followUp.Command)
	assert.JSONEq(t, `{"branches":["feat/x"],"force":true}`, string(followUp.Request))
	assert.Equal(t, provision.CodeLeftBehind, followUp.Code)
}
