package provision_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/storetest"
	"github.com/LucasPcq/wtm/next/internal/app/provision"
	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnEnvThatCannotBeCopiedLeavesNothingBehind(t *testing.T) {
	for name, params := range map[string]func(world, *recorder) provision.Params{
		"a new branch":       world.params,
		"an existing branch": world.existing,
	} {
		t.Run(name, func(t *testing.T) {
			w := onDisk(t)
			before := gitState(t, w)
			source := filepath.Join(w.main, ".env")
			require.NoError(t, os.Chmod(source, 0o000))
			t.Cleanup(func() { _ = os.Chmod(source, 0o644) })
			report := &recorder{}
			p := params(w, report)

			outcome := provision.Run(t.Context(), p)

			assert.Equal(t, wtm.StatusFailed, outcome.Status)
			assert.Empty(t, outcome.Reason)
			assert.Equal(t, "unit.step_failed", outcome.Err.Code)
			assert.Equal(t, wtm.KindInternal, outcome.Err.Kind)
			assert.Equal(t, wtm.Params{"step": "env"}, outcome.Err.Params)
			assert.ErrorContains(t, outcome.Err, "env failed: read ")
			assert.Equal(t, before, gitState(t, w))
			assert.NoDirExists(t, p.Worktree.Path)
			_, err := w.meta.Load(p.Worktree.Branch)
			assert.ErrorIs(t, err, os.ErrNotExist)
			assert.Empty(t, outcome.Provisioned)
			assert.Empty(t, report.worktreeEvents())
		})
	}
}

func TestARecordThatCannotBeWrittenLeavesNothingBehind(t *testing.T) {
	w, files := inMemory(t, seeds{Store: storetest.FakeParams{Fail: map[string]error{"Update": errors.New("disk full")}}})
	before := gitState(t, w)

	outcome := provision.Run(t.Context(), w.params(&recorder{}))

	assert.Equal(t, wtm.StatusFailed, outcome.Status)
	assert.Equal(t, "unit.step_failed", outcome.Err.Code)
	assert.Equal(t, "meta failed: disk full", outcome.Err.Error())
	assert.Equal(t, before, gitState(t, w))
	_, recorded := files.Meta(branchName)
	assert.False(t, recorded)
}

func TestAWorktreeThatCannotBeAddedHasNothingToUndo(t *testing.T) {
	w, files := inMemory(t, seeds{GitFail: map[string]error{"AddWorktree": errors.New("path exists")}})

	outcome := provision.Run(t.Context(), w.params(&recorder{}))

	assert.Equal(t, wtm.StatusFailed, outcome.Status)
	assert.Equal(t, "worktree failed: path exists", outcome.Err.Error())
	assert.Empty(t, files.Calls())
}

func TestAnUndoThatFailsNamesWhatIsLeftAndTheWayOut(t *testing.T) {
	w, _ := inMemory(t, seeds{
		GitFail: map[string]error{"RemoveWorktree": errors.New("worktree is locked")},
		Store:   storetest.FakeParams{Fail: map[string]error{"Update": errors.New("disk full")}},
	})

	outcome := provision.Run(t.Context(), w.params(&recorder{}))

	require.Equal(t, wtm.StatusFailed, outcome.Status)
	assert.Equal(t, "unit.undo_failed", outcome.Err.Code)
	assert.Equal(t, wtm.KindInternal, outcome.Err.Kind)
	assert.Equal(t, "could not remove the worktree, left behind: worktree", outcome.Err.Message)
	assert.Equal(t, wtm.Params{"left": "worktree"}, outcome.Err.Params)
	assert.ErrorContains(t, outcome.Err, "disk full")
	assert.ErrorContains(t, outcome.Err, "worktree is locked")
	followUp := outcome.Err.FollowUp
	require.NotNil(t, followUp)
	assert.Equal(t, "clean", followUp.Command)
	assert.JSONEq(t, `{"branches":["feat/x"],"force":true}`, string(followUp.Request))
	assert.Equal(t, "provision.left_behind", followUp.Code)
	assert.Equal(t, wtm.Params{"branch": branchName, "path": w.path(branchName)}, followUp.Params)
	assert.Equal(t, "branch feat/x is left half-created at "+w.path(branchName)+": remove it with `wtm clean --force feat/x`", followUp.Message)
}

func TestABranchThatCannotBeDeletedIsLeftBehindToo(t *testing.T) {
	w, _ := inMemory(t, seeds{
		GitFail: map[string]error{"DeleteBranch": errors.New("branch is protected")},
		Store:   storetest.FakeParams{Fail: map[string]error{"Update": errors.New("disk full")}},
	})

	outcome := provision.Run(t.Context(), w.params(&recorder{}))

	assert.Equal(t, "unit.undo_failed", outcome.Err.Code)
	assert.Equal(t, "could not delete the branch, left behind: branch", outcome.Err.Message)
	assert.Equal(t, wtm.Params{"left": "branch"}, outcome.Err.Params)
	assert.ErrorContains(t, outcome.Err, "branch is protected")
	followUp := outcome.Err.FollowUp
	require.NotNil(t, followUp)
	assert.Equal(t, "clean", followUp.Command)
	assert.JSONEq(t, `{"branches":["feat/x"],"force":true}`, string(followUp.Request))
	assert.Equal(t, "provision.left_behind", followUp.Code)
	assert.Equal(t, wtm.Params{"branch": branchName}, followUp.Params)
	assert.Equal(t, "branch feat/x is left behind: remove it with `wtm clean --force feat/x`", followUp.Message)
}

func TestACheckOutNeverDeletesTheBranchOnUndo(t *testing.T) {
	w, _ := inMemory(t, seeds{Store: storetest.FakeParams{Fail: map[string]error{"Update": errors.New("disk full")}}})

	outcome := provision.Run(t.Context(), w.existing(&recorder{}))

	assert.Equal(t, "unit.step_failed", outcome.Err.Code)
	assert.Contains(t, gitStateNames(t, w), existing)
}
