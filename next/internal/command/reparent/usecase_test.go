package reparent_test

import (
	"context"
	"os"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/reparent"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAMoveRewritesOnlyTheParentInTheRecord(t *testing.T) {
	w := newWorld(t)
	before, err := w.meta.Load(featC)
	require.NoError(t, err)

	outcome, err := w.usecase.Run(t.Context(), moving("main", featC), domain.RunOptions{})

	require.NoError(t, err)
	require.NotNil(t, outcome.Result)
	assert.Equal(t, []reparent.Item{{Subject: featC, Status: domain.StatusDone, Detail: &reparent.Detail{Path: w.at(featC), From: featB, To: "main"}}}, outcome.Result.Items)
	after, err := w.meta.Load(featC)
	require.NoError(t, err)
	before.SourceBranch = "main"
	assert.Equal(t, before, after)
}

func TestEachMoveIsReportedUpdatedBetweenItsUnitEvents(t *testing.T) {
	w := newWorld(t)
	report := &operationtest.Recorder{}

	_, err := w.usecase.Run(t.Context(), moving("main", featC), domain.RunOptions{Report: report.Add})

	require.NoError(t, err)
	assert.Equal(t, []domain.UnitStarted{{Subject: featC}}, eventsOf[domain.UnitStarted](report.All()))
	updated := eventsOf[domain.WorktreeUpdated](report.All())
	require.Len(t, updated, 1)
	assert.Equal(t, []worktree.IdentityField{worktree.IdentityParent}, updated[0].Changed)
	assert.Equal(t, "main", updated[0].Worktree.Parent)
	assert.Equal(t, []domain.UnitFinished{{Subject: featC, Status: domain.StatusDone}}, eventsOf[domain.UnitFinished](report.All()))
}

func TestAWorktreeAlreadyOnTheNewParentIsLeftAsItIs(t *testing.T) {
	w := newWorld(t)
	report := &operationtest.Recorder{}
	before, err := os.Stat(w.recordPath(t, featX))
	require.NoError(t, err)

	outcome, err := w.usecase.Run(t.Context(), moving("main", featX), domain.RunOptions{Report: report.Add})

	require.NoError(t, err)
	assert.Equal(t, domain.ReasonUnchanged, outcome.Result.Items[0].Reason)
	assert.Equal(t, domain.StatusDone, outcome.Result.Items[0].Status)
	assert.Empty(t, eventsOf[domain.WorktreeUpdated](report.All()))
	assert.Empty(t, outcome.Result.FollowUps)
	after, err := os.Stat(w.recordPath(t, featX))
	require.NoError(t, err)
	assert.Equal(t, before.ModTime(), after.ModTime())
}

func TestTheSyncFollowUpNamesTheWorktreesMovedAndEachHasItsUndo(t *testing.T) {
	w := newWorld(t)
	w.recordParent(t, featA, "")

	outcome, err := w.usecase.Run(t.Context(), moving(topic, featB, featX, featA), domain.RunOptions{})

	require.NoError(t, err)
	assert.Equal(t, []domain.FollowUp{
		{Command: "sync", Answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{featB, featX, featA}}}, Code: reparent.CodeSync, Message: "rebase onto the new parent"},
		{Command: reparent.Name, Answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{featB}}, {Path: domain.PathTo, Values: []string{featA}}}, Code: reparent.CodeUndo, Message: "move feat/b back onto feat/a"},
		{Command: reparent.Name, Answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{featX}}, {Path: domain.PathTo, Values: []string{"main"}}}, Code: reparent.CodeUndo, Message: "move feat/x back onto main"},
	}, outcome.Result.FollowUps)
}

func TestNoUndoBackOntoAParentThatIsNoBranch(t *testing.T) {
	w := newWorld(t)
	w.recordParent(t, featX, "feat/deleted")

	outcome, err := w.usecase.Run(t.Context(), moving("main", featX), domain.RunOptions{})

	require.NoError(t, err)
	require.Len(t, outcome.Result.FollowUps, 1)
	assert.Equal(t, reparent.CodeSync, outcome.Result.FollowUps[0].Code)
}

func TestNoUndoBackIntoTheCycleAWorktreeWasMovedOutOf(t *testing.T) {
	w := newWorld(t)
	w.recordParent(t, featA, featB)

	outcome, err := w.usecase.Run(t.Context(), moving("main", featA), domain.RunOptions{})

	require.NoError(t, err)
	require.Len(t, outcome.Result.FollowUps, 1)
	assert.Equal(t, reparent.CodeSync, outcome.Result.FollowUps[0].Code)
}

func TestARecordThatCannotBeWrittenFailsAloneAndStopsNoOther(t *testing.T) {
	w := newWorld(t)
	w.unwritable(t, featB)

	outcome, err := w.usecase.Run(t.Context(), moving(topic, featB, featX), domain.RunOptions{})

	require.NoError(t, err)
	failed := outcome.Result.Items[0]
	assert.Equal(t, domain.StatusFailed, failed.Status)
	require.NotNil(t, failed.Err)
	assert.Equal(t, reparent.CodeWriteFailed, failed.Err.Code)
	assert.NotEmpty(t, failed.Err.Params["cause"], "the surface shows why the record did not write")
	assert.Equal(t, featA, w.parentOf(t, featB))
	assert.Equal(t, domain.StatusDone, outcome.Result.Items[1].Status)
	assert.Equal(t, topic, w.parentOf(t, featX))
}

func TestACancelledApplySkipsTheWorktreesNotStarted(t *testing.T) {
	w := newWorld(t)
	plan, _, err := w.usecase.Plan(t.Context(), moving("main", featB, featC))
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	result := w.usecase.Apply(ctx, plan, func(domain.Event) {})

	for _, item := range result.Items {
		assert.Equal(t, domain.StatusSkipped, item.Status)
		assert.Equal(t, domain.ReasonInterrupted, item.Reason)
	}
	assert.Equal(t, featA, w.parentOf(t, featB))
}

func TestAnotherOperationHoldingTheNewParentsChainRefusesTheMove(t *testing.T) {
	w := newWorld(t)
	held, err := w.lock(t, featA)
	require.NoError(t, err)
	defer held.Release()

	_, err = w.usecase.Run(t.Context(), moving(featB, featC), domain.RunOptions{})

	assert.Equal(t, "lock.held", failureOf(t, err).Code)
	assert.Equal(t, featB, w.parentOf(t, featC))
}

func TestThePlanIsRefusedOnceAParentChangedUnderIt(t *testing.T) {
	w := newWorld(t)
	preview, err := w.usecase.Run(t.Context(), moving("main", featC), domain.RunOptions{DryRun: true})
	require.NoError(t, err)
	w.recordParent(t, featC, featA)

	_, err = w.usecase.Run(t.Context(), moving("main", featC), domain.RunOptions{ExpectFingerprint: preview.Fingerprint})

	assert.True(t, domain.PlanStale(err))
	assert.Equal(t, featA, w.parentOf(t, featC))
}

func TestAWorktreeWhoseRecordANewerWtmWroteIsRefusedAsAWriteIs(t *testing.T) {
	w := newWorld(t)
	newer := `{"schema": 99, "source_branch": "main"}`
	writeFile(t, w.recordPath(t, featX), newer)

	_, err := w.usecase.Run(t.Context(), moving("main", featX), domain.RunOptions{})

	refused := failureOf(t, err)
	assert.Equal(t, reparent.CodeMetaNewer, refused.Code)
	assert.Equal(t, domain.KindPrecondition, refused.Kind)
	assert.Contains(t, refused.Message, featX)
	var schema *domain.SchemaNewer
	assert.ErrorAs(t, err, &schema)
	content, readErr := os.ReadFile(w.recordPath(t, featX))
	require.NoError(t, readErr)
	assert.Equal(t, newer, string(content))
}
