package fastforward_test

import (
	"context"
	"maps"
	"slices"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/command/fastforward"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEveryWorktreeStandsWhereOriginHasItNow(t *testing.T) {
	w := newWorld(t)

	planned, _ := w.plan(t, fastforward.Request{All: true})

	assert.Equal(t, map[string]fastforward.Standing{
		"main":      fastforward.StandingBehind,
		behind:      fastforward.StandingBehind,
		stale:       fastforward.StandingBehind,
		ahead:       fastforward.StandingAhead,
		diverged:    fastforward.StandingDiverged,
		dirty:       fastforward.StandingBehind,
		dirtyLatest: fastforward.StandingUpToDate,
		latest:      fastforward.StandingUpToDate,
		unpushed:    fastforward.StandingNoOrigin,
		rebasing:    fastforward.StandingBusy,
		bisecting:   fastforward.StandingBusy,
		picking:     fastforward.StandingBusy,
		gone:        fastforward.StandingMissing,
	}, standings(planned))
	assert.Equal(t, "main", planned.Worktrees[0].Worktree)
}

func TestEachWorktreeBehindOriginMovesAndTheOthersStandAsTheyAre(t *testing.T) {
	w := newWorld(t)
	before, from := w.tip(t, latest), w.tip(t, behind)

	outcome, _ := w.run(t, fastforward.Request{All: true})

	items := itemsOf(outcome)
	for _, name := range []string{"main", behind, stale} {
		assert.Equal(t, domain.StatusDone, items[name].Status, name)
		assert.True(t, w.moved(t, name), name)
	}
	for _, name := range []string{ahead, latest, dirtyLatest} {
		assert.Equal(t, domain.ReasonUnchanged, items[name].Reason, name)
	}
	for _, name := range []string{diverged, dirty, unpushed, rebasing, bisecting, picking, gone} {
		assert.Equal(t, domain.StatusSkipped, items[name].Status, name)
		assert.Equal(t, domain.ReasonRuledOut, items[name].Reason, name)
		if name != unpushed {
			assert.False(t, w.moved(t, name), name)
		}
	}
	assert.Equal(t, before, w.tip(t, latest))
	assert.Equal(t, &fastforward.Detail{Path: w.at(behind), From: from, To: w.tip(t, behind)}, items[behind].Detail)
}

func TestAWorktreeInTheMiddleOfAnOperationIsLeftAsItIsAndNamesHowToFinishIt(t *testing.T) {
	for name, operation := range map[string]worktree.Operation{rebasing: worktree.OperationRebase, bisecting: worktree.OperationBisect, picking: worktree.OperationCherryPick} {
		t.Run(worktree.PathSegment(name), func(t *testing.T) {
			w := newWorld(t)
			head, branch := gittest.Output(t, w.at(name), "rev-parse", "HEAD"), w.tip(t, name)

			outcome, report := w.run(t, naming(name))

			assert.Equal(t, domain.ReasonRuledOut, itemsOf(outcome)[name].Reason)
			assert.Equal(t, operation, outcome.Plan.Worktrees[0].Operation)
			assert.Equal(t, branch, w.tip(t, name))
			assert.Equal(t, head, gittest.Output(t, w.at(name), "rev-parse", "HEAD"))
			assert.Empty(t, eventsOf[domain.PhaseStarted](report.All()))
			require.Equal(t, []string{fastforward.CodeBusy}, codes(outcome.Warnings))
			assert.Equal(t, domain.CommandGo, outcome.Warnings[0].FollowUp.Command)
		})
	}
}

func TestADirtyWorktreeWithNothingToMoveIsNoRefusal(t *testing.T) {
	w := newWorld(t)

	outcome, _ := w.run(t, naming(dirtyLatest))

	assert.Equal(t, domain.ReasonUnchanged, itemsOf(outcome)[dirtyLatest].Reason)
	assert.Equal(t, []fastforward.Blocker{}, outcome.Plan.Worktrees[0].Blockers)
}

func TestANamedDirtyWorktreeBehindOriginRefusesTheRequestAndNamesEachOne(t *testing.T) {
	w := newWorld(t)
	w.dirtied(t, behind)
	before := w.tip(t, stale)

	_, err := w.usecase.Run(t.Context(), naming(stale, dirty, behind), domain.RunOptions{})

	refused := failureOf(t, err)
	assert.Equal(t, domain.KindRefused, refused.Kind)
	assert.Equal(t, fastforward.CodeUnsafe, refused.Code)
	assert.Equal(t, map[string][]string{"worktree[1]": {fastforward.CodeDirty}, "worktree[2]": {fastforward.CodeDirty}}, fieldCodes(refused.Fields))
	assert.Equal(t, fastforward.CodeForce, refused.FollowUp.Code)
	assert.Contains(t, refused.FollowUp.Answers, domain.Answered{Path: fastforward.PathForce, Values: []string{"true"}})
	assert.Equal(t, before, w.tip(t, stale))
}

func TestADirtyWorktreeTheCommandChoseIsKeptWithOneRequestForcingEveryOne(t *testing.T) {
	w := newWorld(t)
	w.dirtied(t, behind)

	outcome, _ := w.run(t, fastforward.Request{All: true})

	kept := keptOf(outcome.Warnings)
	require.Len(t, kept, 2)
	assert.Equal(t, domain.Params{"worktree": behind, "blockers": "dirty"}, kept[0].Params)
	assert.Nil(t, kept[0].FollowUp)
	assert.Equal(t, fastforward.CodeForce, kept[1].FollowUp.Code)
	assert.Equal(t, []domain.Answered{{Path: domain.PathWorktree, Values: []string{behind, dirty}}, {Path: fastforward.PathForce, Values: []string{"true"}}}, kept[1].FollowUp.Answers)
	for _, name := range []string{behind, dirty} {
		assert.Equal(t, domain.ReasonRuledOut, itemsOf(outcome)[name].Reason, name)
	}
}

func TestNamingNoneMovesTheWorktreesBehindOriginAndKeepsTheDirtyOnes(t *testing.T) {
	w := newWorld(t)

	outcome, _ := w.run(t, fastforward.Request{})

	assert.Equal(t, []string{"main", behind, dirty, stale}, subjectsOf(outcome))
	assert.Equal(t, []string{"main", behind, stale}, moving(outcome.Plan))
	assert.Equal(t, []fastforward.Blocker{fastforward.BlockerDirty}, outcome.Plan.Worktrees[2].Blockers)
	assert.Equal(t, domain.ReasonRuledOut, itemsOf(outcome)[dirty].Reason)
}

func TestForceMovesADirtyWorktree(t *testing.T) {
	w := newWorld(t)

	outcome, _ := w.run(t, fastforward.Request{Worktrees: []string{dirty}, Force: true})

	assert.Equal(t, domain.StatusDone, itemsOf(outcome)[dirty].Status)
	assert.True(t, w.moved(t, dirty))
	assert.Equal(t, fastforward.NoteForced, outcome.Plan.Recap[0].Values[0].Note)
}

func TestAWorktreeAheadOfOriginIsSaidAheadNotUpToDate(t *testing.T) {
	w := newWorld(t)

	outcome, _ := w.run(t, naming(ahead))

	assert.Equal(t, fastforward.StandingAhead, outcome.Plan.Worktrees[0].Standing)
	assert.Equal(t, domain.ReasonUnchanged, itemsOf(outcome)[ahead].Reason)
	assert.Equal(t, "feat/ahead (ahead of origin)", outcome.Plan.Recap[0].Values[0].Text)
}

func TestADivergedWorktreeIsLeftWithTheSyncThatRebasesIt(t *testing.T) {
	w := newWorld(t)

	outcome, _ := w.run(t, naming(diverged))

	require.Equal(t, []string{fastforward.CodeDiverged}, codes(outcome.Warnings))
	warning := outcome.Warnings[0]
	assert.Equal(t, domain.Params{"worktree": diverged, "ahead": "1", "behind": "1"}, warning.Params)
	assert.Equal(t, "sync", warning.FollowUp.Command)
	assert.Equal(t, []domain.Answered{{Path: domain.PathWorktree, Values: []string{diverged}}}, warning.FollowUp.Answers)
}

func TestTheFetchSeesWhatOriginHasNow(t *testing.T) {
	w := newWorld(t)
	before := w.questions(t, naming(stale))
	require.Equal(t, worktree.UpToDate, optionOf(before.Worktrees.Options, stale).Origin.State)

	outcome, _ := w.run(t, naming(stale))

	assert.Equal(t, fastforward.StandingBehind, outcome.Plan.Worktrees[0].Standing)
	assert.True(t, w.moved(t, stale))
}

func TestAFetchThatFailsIsAWarningAndThePlanStandsOnOriginAsLastFetched(t *testing.T) {
	w := newWorld(t)
	w.usecase.Git = unfetchable{w.repo}

	planned, warnings := w.plan(t, naming(behind, stale))

	assert.Equal(t, []string{fastforward.CodeFetchFailed}, codes(warnings))
	assert.Equal(t, []string{behind}, moving(planned))
}

func TestAWorktreeGitRefusesToMoveFailsAloneWithGitsOwnWords(t *testing.T) {
	w := newWorld(t)
	w.usecase.Provision.Git = refusingMerge{Repo: w.repo, path: w.at(behind)}

	outcome, _ := w.run(t, naming(behind, "main"))

	failed := itemsOf(outcome)[behind]
	assert.Equal(t, domain.StatusFailed, failed.Status)
	assert.Equal(t, fastforward.CodeFailed, failed.Err.Code)
	assert.Equal(t, "git refused to move the branch", failed.Err.Message)
	assert.Equal(t, domain.Params{"branch": behind, "path": w.at(behind), "cause": errRefused.Error()}, failed.Err.Params)
	assert.Equal(t, domain.StatusDone, itemsOf(outcome)["main"].Status)
	assert.True(t, w.moved(t, "main"))
}

func TestTheApplyLandsOnTheCommitPlannedThoughOriginMovedSince(t *testing.T) {
	w := newWorld(t)
	planned, _ := w.plan(t, naming(behind))
	w.pushed(t, behind)

	result := w.usecase.Apply(t.Context(), planned, (&operationtest.Recorder{}).Add)

	assert.Equal(t, domain.StatusDone, result.Items[0].Status)
	assert.Equal(t, planned.Worktrees[0].To, w.tip(t, behind))
	assert.Equal(t, planned.Worktrees[0].To, result.Items[0].Detail.To)
	assert.NotEqual(t, w.tip(t, behind), gittest.Output(t, w.main, "ls-remote", "origin", "refs/heads/"+behind)[:40])
}

func TestAWorktreeStartedInARebaseSinceThePlanFailsAloneAndSaysSo(t *testing.T) {
	w := newWorld(t)
	planned, _ := w.plan(t, naming(behind, "main"))
	w.stopInARebase(t, behind)

	result := w.usecase.Apply(t.Context(), planned, (&operationtest.Recorder{}).Add)

	assert.Equal(t, domain.StatusFailed, result.Items[0].Status)
	assert.Equal(t, fastforward.CodeBusy, result.Items[0].Err.Code)
	assert.Equal(t, "the worktree is in the middle of an operation", result.Items[0].Err.Message)
	assert.Equal(t, domain.StatusDone, result.Items[1].Status)
}

func TestAWorktreeDirtiedSinceThePlanFailsAloneAndSaysSo(t *testing.T) {
	w := newWorld(t)
	planned, _ := w.plan(t, naming(behind, "main"))
	w.dirtied(t, behind)

	result := w.usecase.Apply(t.Context(), planned, (&operationtest.Recorder{}).Add)

	assert.Equal(t, domain.StatusFailed, result.Items[0].Status)
	assert.Equal(t, fastforward.CodeDirty, result.Items[0].Err.Code)
	assert.Equal(t, "the worktree has uncommitted changes", result.Items[0].Err.Message)
	assert.Equal(t, domain.StatusDone, result.Items[1].Status)
	assert.False(t, w.moved(t, behind))
}

func TestANamedWorktreeIsTheOnlyOneRead(t *testing.T) {
	w := newWorld(t)

	facts, err := w.usecase.Observe(t.Context(), naming(behind))

	require.NoError(t, err)
	assert.Equal(t, []string{behind}, slices.Collect(maps.Keys(facts.Checks)))
}

func TestOnlyTheWorktreesThatMoveReportTheirUnitAndPhase(t *testing.T) {
	w := newWorld(t)

	_, report := w.run(t, naming(latest, behind, diverged))

	started := eventsOf[domain.UnitStarted](report.All())
	require.Len(t, started, 1)
	assert.Equal(t, behind, started[0].Subject)
	assert.Equal(t, []domain.PhaseStarted{{Subject: behind, Phase: domain.PhaseFastForward}}, eventsOf[domain.PhaseStarted](report.All()))
}

func TestACancelledRunSkipsTheWorktreesNotStarted(t *testing.T) {
	w := newWorld(t)
	planned, _ := w.plan(t, naming(behind, "main"))
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	result := w.usecase.Apply(ctx, planned, (&operationtest.Recorder{}).Add)

	for _, item := range result.Items {
		assert.Equal(t, domain.ReasonInterrupted, item.Reason, item.Subject)
	}
	assert.False(t, w.moved(t, behind))
}

func TestThePlanLocksTheWorktreesItMoves(t *testing.T) {
	w := newWorld(t)

	planned, _ := w.plan(t, naming(behind, latest))

	assert.Equal(t, domain.WorktreeLocks([]string{behind}), planned.Locks())
}

func TestOriginMovedSinceTheDryRunMakesItsPlanStale(t *testing.T) {
	w := newWorld(t)
	preview, err := w.usecase.Run(t.Context(), naming(behind), domain.RunOptions{DryRun: true})
	require.NoError(t, err)
	w.pushed(t, behind)

	_, err = w.usecase.Run(t.Context(), naming(behind), domain.RunOptions{ExpectFingerprint: preview.Fingerprint})

	assert.True(t, domain.PlanStale(err))
}

func TestNoFetchPlansOnOriginAsLastFetched(t *testing.T) {
	w := newWorld(t)
	before := w.tip(t, stale)
	req := naming(stale)
	req.NoFetch = true

	outcome, _ := w.run(t, req)

	assert.Equal(t, fastforward.StandingUpToDate, outcome.Plan.Worktrees[0].Standing)
	assert.Equal(t, before, w.tip(t, stale))
}

func TestTheForcedFollowUpsLeaveNoFetchOut(t *testing.T) {
	w := newWorld(t)
	noFetch := domain.Answered{Path: fastforward.PathNoFetch, Values: []string{"true"}}

	_, err := w.usecase.Run(t.Context(), fastforward.Request{Worktrees: []string{dirty}, NoFetch: true}, domain.RunOptions{})
	outcome, _ := w.run(t, fastforward.Request{All: true, NoFetch: true})

	assert.NotContains(t, failureOf(t, err).FollowUp.Answers, noFetch)
	kept := keptOf(outcome.Warnings)
	require.NotEmpty(t, kept)
	assert.NotContains(t, kept[len(kept)-1].FollowUp.Answers, noFetch)
}
