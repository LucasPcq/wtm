package sync_test

import (
	"context"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/command/sync"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAWorktreeNamedIsRebasedWithEveryWorktreeUnderItParentsFirst(t *testing.T) {
	w := newWorld(t)
	report := &operationtest.Recorder{}

	outcome, err := w.usecase.Run(t.Context(), naming(featA), domain.RunOptions{Report: report.Add})

	require.NoError(t, err)
	assert.Equal(t, []string{featA, featB, featC}, subjectsOf(outcome))
	for _, branch := range []string{featA, featB, featC} {
		assert.Equal(t, domain.StatusDone, itemsOf(outcome)[branch].Status, branch)
	}
	assert.True(t, w.holds(t, featC, "main"))
	assert.Equal(t, w.tip(t, featB), w.tip(t, featC+"~1"), "feat/c replays its own commit alone, on feat/b rebased")
	assert.Equal(t, w.tip(t, featA), w.tip(t, featB+"~1"))
	assert.Equal(t, []int{1, 2, 3}, []int{rebaseOf(outcome.Plan, featA).Depth, rebaseOf(outcome.Plan, featB).Depth, rebaseOf(outcome.Plan, featC).Depth})
	assert.Empty(t, outcome.Plan.LeftOut)
	assert.False(t, rebaseOf(outcome.Plan, featA).Descendant)
	assert.True(t, rebaseOf(outcome.Plan, featC).Descendant)
	var started []string
	for _, event := range report.All() {
		if unit, ok := event.(domain.UnitStarted); ok {
			started = append(started, unit.Subject)
		}
	}
	assert.Equal(t, []string{featA, featB, featC}, started)
}

func TestWithoutDescendantsTheNamedWorktreeAloneIsRebased(t *testing.T) {
	w := newWorld(t)
	b := w.tip(t, featB)
	req := naming(featA)
	req.Descendants = new(false)

	outcome := w.run(t, req)

	assert.Equal(t, []string{featA}, subjectsOf(outcome))
	assert.Equal(t, b, w.tip(t, featB))
	assert.Equal(t, []string{featB, featC}, outcome.Plan.LeftOut)
	assert.Equal(t, "only the named: feat/b, feat/c stay where they are", outcome.Plan.Recap[0].Values[0].Text)
	leftOut := outcome.Result.FollowUps[slices.IndexFunc(outcome.Result.FollowUps, func(f domain.FollowUp) bool { return f.Code == sync.CodeLeftOut })]
	assert.Equal(t, []domain.Answered{{Path: domain.PathWorktree, Values: []string{featB, featC}}}, leftOut.Answers, "their own sync, the core's to spell")
}

func TestAConflictMidStackIsAbortedAndTheWorktreesUnderItSkipped(t *testing.T) {
	w := newWorld(t)
	w.conflicting(t)
	b := w.tip(t, featB)

	outcome := w.run(t, naming(featA))

	items := itemsOf(outcome)
	assert.Equal(t, domain.StatusDone, items[featA].Status)
	require.Equal(t, domain.StatusFailed, items[featB].Status)
	assert.Equal(t, sync.CodeConflict, items[featB].Err.Code)
	assert.Equal(t, domain.KindConflict, items[featB].Err.Kind)
	assert.Equal(t, "shared.txt", items[featB].Err.Params["files"])
	assert.Equal(t, "false", items[featB].Err.Params["kept"])
	assert.Equal(t, domain.StatusSkipped, items[featC].Status)
	assert.Equal(t, domain.ReasonBlocked, items[featC].Reason)
	assert.Equal(t, b, w.tip(t, featB), "the conflict is aborted: feat/b stands where it did")
	assert.Empty(t, gittest.Output(t, w.at(featB), "status", "--porcelain"))
	assert.Equal(t, []string{sync.CodeResolve, sync.CodePush}, followUpCodes(outcome.Result.FollowUps))
}

// B1 and B4: the legacy sync read a branch whose conflict was resolved by hand as diverged from origin for ever, and replayed its parent's old commits into its children.
func TestAConflictLeftInProgressIsSyncedOnceResolvedWithEverythingUnderIt(t *testing.T) {
	w := newWorld(t)
	w.conflicting(t)
	gittest.Git(t, w.main, "push", "-q", "-f", "origin", featB, featC)
	req := naming(featA)
	req.KeepConflict = true

	kept := w.run(t, req)
	require.Equal(t, "true", itemsOf(kept)[featB].Err.Params["kept"])
	assert.DirExists(t, filepath.Join(gittest.Output(t, w.at(featB), "rev-parse", "--absolute-git-dir"), "rebase-merge"))
	assert.Equal(t, []string{sync.CodeGo, sync.CodeResume, sync.CodePush}, followUpCodes(kept.Result.FollowUps))
	w.resolve(t, featB)

	resumed := w.run(t, naming(featB))

	items := itemsOf(resumed)
	assert.Equal(t, sync.StandingOnParent, rebaseOf(resumed.Plan, featB).Standing, "feat/b holds every commit origin has: not diverged")
	assert.Equal(t, domain.StatusDone, items[featC].Status, items[featC].Err)
	assert.Equal(t, w.tip(t, featB), w.tip(t, featC+"~1"), "feat/c replays its own commit alone, none of feat/b's old ones")
}

// B2: the legacy sync read a worktree on no branch as a cycle and refused every run.
func TestEveryWorktreeIsSyncedBesideOneOnNoBranchAndEachLeftOneSaysWhy(t *testing.T) {
	w := newWorld(t)

	outcome, err := w.usecase.Run(t.Context(), sync.Request{All: true}, domain.RunOptions{})

	require.NoError(t, err)
	items := itemsOf(outcome)
	for _, branch := range []string{featA, featB, featC, solo} {
		assert.Equal(t, domain.StatusDone, items[branch].Status, branch)
	}
	assert.Equal(t, map[string]sync.Standing{featA: sync.StandingRebases, featB: sync.StandingRebases, featC: sync.StandingRebases, solo: sync.StandingRebases, dirty: sync.StandingRebases, dirtyKid: sync.StandingParentLeft, busy: sync.StandingBusy, gone: sync.StandingMissing, orphan: sync.StandingParentGone, raw: sync.StandingNoParent}, standings(outcome.Plan))
	assert.ElementsMatch(t, []string{sync.CodeParentLeft, sync.CodeBusy, sync.CodeMissing, sync.CodeParentGone, sync.CodeNoParent, sync.CodeKept}, codes(outcome.Warnings))
	assert.Equal(t, domain.ReasonRuledOut, items[dirty].Reason, "a dirty worktree the command chose is kept")
}

// B3: the legacy dry run read origin as last fetched and planned another run than the one it previewed.
func TestADryRunFetchesOriginAndPlansWhatTheApplyDoes(t *testing.T) {
	w := newWorld(t)
	w.pushedOnOrigin(t, "main")

	preview, err := w.usecase.Run(t.Context(), naming(solo), domain.RunOptions{DryRun: true})
	require.NoError(t, err)
	require.Len(t, preview.Plan.Onto, 1)
	assert.Equal(t, []domain.Phase{domain.PhaseFastForward}, preview.Plan.Onto[0].Phases, "main is behind origin, once fetched")

	applied, err := w.usecase.Run(t.Context(), naming(solo), domain.RunOptions{ExpectFingerprint: preview.Fingerprint})

	require.NoError(t, err)
	assert.Equal(t, domain.StatusDone, itemsOf(applied)[solo].Status)
	assert.Equal(t, w.tip(t, "origin/main"), w.tip(t, "main"))
	assert.True(t, w.holds(t, solo, "origin/main"))
}

// B5: the legacy sync refused every run while a cycle stood anywhere in the stack.
func TestACycleRefusesOnlyTheSyncThatNamesIt(t *testing.T) {
	w := newWorld(t)
	w.recordParent(t, featA, featC)

	elsewhere, err := w.usecase.Run(t.Context(), naming(solo), domain.RunOptions{})
	require.NoError(t, err)
	assert.Equal(t, domain.StatusDone, itemsOf(elsewhere)[solo].Status)

	_, err = w.usecase.Run(t.Context(), naming(featB), domain.RunOptions{})

	refused := failureOf(t, err)
	assert.Equal(t, sync.CodeCycle, refused.Code)
	assert.Equal(t, domain.KindRefused, refused.Kind)
	require.NotNil(t, refused.FollowUp)
	assert.Equal(t, domain.CommandReparent, refused.FollowUp.Command)
}

// B7: the legacy sync left a dirty base behind origin without a word and rebased onto it.
func TestADirtyBaseBehindOriginIsKeptWithAWarning(t *testing.T) {
	w := newWorld(t)
	w.pushedOnOrigin(t, "main")
	writeFile(t, filepath.Join(w.main, "main.txt"), "changed\n")

	planned, warnings := w.plan(t, naming(solo))

	require.Len(t, planned.Onto, 1)
	assert.Equal(t, []sync.Blocker{sync.BlockerDirty}, planned.Onto[0].Blockers)
	assert.Empty(t, planned.Onto[0].Phases)
	assert.Contains(t, codes(warnings), sync.CodeKept)

	outcome := w.run(t, sync.Request{Worktrees: []string{solo}, NoFetch: true})

	require.Equal(t, domain.StatusDone, itemsOf(outcome)[solo].Status, itemsOf(outcome)[solo].Err)
	assert.Equal(t, w.tip(t, "main"), w.tip(t, solo+"~1"), "onto main as it stands, not origin's main it was kept from")
}

// B8: the legacy sync failed on a worktree whose directory is gone with git's own words.
func TestAWorktreeWhoseDirectoryIsGoneIsLeftWithItsCleanFollowUp(t *testing.T) {
	w := newWorld(t)

	outcome := w.run(t, naming(gone))

	assert.Equal(t, domain.ReasonRuledOut, itemsOf(outcome)[gone].Reason)
	require.Equal(t, []string{sync.CodeMissing}, codes(outcome.Warnings))
	assert.Equal(t, domain.CommandClean, outcome.Warnings[0].FollowUp.Command)
}

func TestADirtyWorktreeNamedRefusesTheRunBeforeAnythingMoves(t *testing.T) {
	w := newWorld(t)
	a := w.tip(t, featA)

	_, err := w.usecase.Run(t.Context(), naming(featA, dirty), domain.RunOptions{})

	refused := failureOf(t, err)
	assert.Equal(t, sync.CodeUnsafe, refused.Code)
	assert.Equal(t, sync.CodeDirty, refused.Fields[0].Code)
	assert.Equal(t, "worktree[1]", refused.Fields[0].Path)
	assert.Equal(t, sync.CodeForce, refused.FollowUp.Code)
	assert.Equal(t, a, w.tip(t, featA))
}

func TestForceRebasesADirtyWorktreeWithItsChangesStashedAroundTheRebase(t *testing.T) {
	w := newWorld(t)
	req := naming(dirty)
	req.Force = true

	outcome := w.run(t, req)

	assert.Equal(t, domain.StatusDone, itemsOf(outcome)[dirty].Status, itemsOf(outcome)[dirty].Err)
	assert.Equal(t, domain.StatusDone, itemsOf(outcome)[dirtyKid].Status, "the worktree under it follows")
	assert.True(t, w.holds(t, dirty, "main"))
	assert.FileExists(t, filepath.Join(w.at(dirty), "wip.txt"))
}

func TestADirtyDescendantIsKeptAndTheWorktreesUnderItLeft(t *testing.T) {
	w := newWorld(t)
	w.commit(t, featA, "more.txt", "more")
	writeFile(t, filepath.Join(w.at(featB), "wip.txt"), "work\n")

	outcome := w.run(t, naming(featA))

	assert.Equal(t, domain.StatusDone, itemsOf(outcome)[featA].Status)
	assert.Equal(t, domain.ReasonRuledOut, itemsOf(outcome)[featB].Reason)
	assert.Equal(t, sync.StandingParentLeft, rebaseOf(outcome.Plan, featC).Standing)
	assert.Equal(t, []string{sync.CodeParentLeft, sync.CodeKept}, codes(outcome.Warnings))
	assert.Equal(t, sync.CodeForce, outcome.Warnings[1].FollowUp.Code)
}

func TestPushForcePushesEachBranchSyncedOverTheTipPlanned(t *testing.T) {
	w := newWorld(t)
	req := naming(featA)
	req.Push = true

	outcome := w.run(t, req)

	for _, branch := range []string{featA, featB, featC} {
		require.Equal(t, domain.StatusDone, itemsOf(outcome)[branch].Status, itemsOf(outcome)[branch].Err)
		assert.True(t, itemsOf(outcome)[branch].Detail.Pushed, branch)
		assert.Equal(t, w.tip(t, branch), gittest.Output(t, w.main, "ls-remote", "origin", "refs/heads/"+branch)[:40], branch)
	}
	assert.Empty(t, outcome.Result.FollowUps)
}

func TestATeammatesPushSinceTheDryRunMakesItsPlanStale(t *testing.T) {
	w := newWorld(t)
	req := naming(solo)
	req.Push = true
	planned, err := w.usecase.Run(t.Context(), req, domain.RunOptions{DryRun: true})
	require.NoError(t, err)
	w.pushedOnOrigin(t, solo)

	outcome, err := w.usecase.Run(t.Context(), req, domain.RunOptions{ExpectFingerprint: planned.Fingerprint})

	require.True(t, domain.PlanStale(err), "origin moved: the plan is stale, %v", err)
	assert.Nil(t, outcome.Result)
}

func TestACancelledSyncSkipsTheWorktreesNotStarted(t *testing.T) {
	w := newWorld(t)
	planned, _ := w.plan(t, naming(featA))
	a := w.tip(t, featA)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	result := w.usecase.Apply(ctx, planned, (&operationtest.Recorder{}).Add)

	require.Len(t, result.Items, 3)
	for _, item := range result.Items {
		assert.Equal(t, domain.ReasonInterrupted, item.Reason, item.Subject)
	}
	assert.Equal(t, a, w.tip(t, featA))
}

func TestAPushAloneIsAChangeTheItemSaysIsDone(t *testing.T) {
	w := newWorld(t)
	w.run(t, naming(solo))
	req := naming(solo)
	req.Push = true

	outcome := w.run(t, req)

	item := itemsOf(outcome)[solo]
	assert.Equal(t, []domain.Phase{domain.PhasePush}, rebaseOf(outcome.Plan, solo).Phases)
	assert.Equal(t, domain.StatusDone, item.Status)
	assert.Empty(t, item.Reason)
	assert.True(t, item.Detail.Pushed)
}

func TestAPushRefusedAtApplyFailsTheItemKeepsItsRebaseAndStopsTheWorktreesUnderIt(t *testing.T) {
	w := newWorld(t)
	w.usecase.Git = refusingPush{Repo: w.repo}
	req := naming(featA)
	req.Push = true

	outcome := w.run(t, req)

	items := itemsOf(outcome)
	require.Equal(t, domain.StatusFailed, items[featA].Status)
	assert.Equal(t, sync.CodePushFailed, items[featA].Err.Code)
	require.NotNil(t, items[featA].Detail, "the rebase that went through is reported")
	assert.False(t, items[featA].Detail.Pushed)
	assert.True(t, w.holds(t, featA, "main"))
	assert.Equal(t, domain.ReasonBlocked, items[featB].Reason)
}

func TestAFirstCancelLetsTheRebaseFinishAndStartsNoPush(t *testing.T) {
	w := newWorld(t)
	req := naming(solo)
	req.Push = true
	planned, _ := w.plan(t, req)
	ctx, cancel := context.WithCancel(t.Context())
	w.usecase.Git = cancellingRebase{Repo: w.repo, cancel: cancel}

	result := w.usecase.Apply(ctx, planned, (&operationtest.Recorder{}).Add)

	item := result.Items[0]
	assert.Equal(t, domain.StatusCancelled, item.Status)
	assert.Equal(t, domain.ReasonInterrupted, item.Reason)
	assert.True(t, w.holds(t, solo, "main"), "the rebase under way finished")
	assert.NotEqual(t, w.tip(t, solo), gittest.Output(t, w.main, "ls-remote", "origin", "refs/heads/"+solo)[:40], "nothing pushed")
}

func TestTheRebaseGoesOntoTheParentAsPlannedNotAsItStandsLater(t *testing.T) {
	w := newWorld(t)
	planned, _ := w.plan(t, naming(solo))
	onto := w.tip(t, "main")
	w.commit(t, "main", "later.txt", "after the plan")

	w.usecase.Apply(t.Context(), planned, (&operationtest.Recorder{}).Add)

	assert.Equal(t, onto, w.tip(t, solo+"~1"))
}

func TestABranchRebasedOntoThatNoWorktreeHoldsMovesToTheCommitPlanned(t *testing.T) {
	w := newWorld(t)
	gittest.Git(t, w.main, "branch", "topic", "main")
	gittest.Git(t, w.main, "push", "-q", "origin", "topic")
	w.pushedOnOrigin(t, "topic")
	gittest.Git(t, w.main, "fetch", "-q", "origin")
	planned := w.tip(t, "origin/topic")
	w.pushedOnOrigin(t, "topic")
	// a push from here moves origin/topic too: back to the fetch the plan stands on
	gittest.Git(t, w.main, "update-ref", "refs/remotes/origin/topic", planned)
	w.recordParent(t, solo, "topic")

	outcome, err := w.usecase.Run(t.Context(), sync.Request{Worktrees: []string{solo}, NoFetch: true}, domain.RunOptions{})

	require.NoError(t, err)
	assert.Equal(t, domain.StatusDone, itemsOf(outcome)[solo].Status, itemsOf(outcome)[solo].Err)
	assert.Equal(t, planned, w.tip(t, "topic"), "origin's later push is the next run's")
	assert.True(t, w.holds(t, solo, "topic"))
}

// A worktree that shares no commit with its parent failed every plan while its fork point was read for every worktree.
func TestAWorktreeSharingNoCommitWithItsParentIsLeftAndFailsNoOtherSync(t *testing.T) {
	w := newWorld(t)
	gittest.Git(t, w.at(orphan), "checkout", "-q", "--orphan", "feat/alone")
	gittest.Git(t, w.at(orphan), "commit", "-q", "--allow-empty", "-m", "unrelated")
	w.record(t, "feat/alone", worktree.Meta{SourceBranch: "main", CreatedAt: createdAt.Format(time.RFC3339)})

	elsewhere := w.run(t, naming(solo))
	all, err := w.usecase.Run(t.Context(), sync.Request{All: true}, domain.RunOptions{})

	assert.Equal(t, domain.StatusDone, itemsOf(elsewhere)[solo].Status)
	require.NoError(t, err)
	assert.Equal(t, sync.StandingUnrelated, standings(all.Plan)["feat/alone"])
	assert.Contains(t, codes(all.Warnings), sync.CodeUnrelated)
}

// The fork point read from the parent's reflog counts a commit the parent held then lost to a reset as the parent's: rebasing from it dropped the commit from the child.
func TestACommitTheParentLostToAResetStaysInTheChild(t *testing.T) {
	w := newWorld(t)
	w.commit(t, "main", "mistake.txt", "meant for solo")
	w.startFrom(t, solo, "main")
	w.commit(t, solo, "solo2.txt", "more")
	gittest.Git(t, w.main, "reset", "-q", "--hard", "HEAD~1")
	w.commit(t, "main", "fix.txt", "the real next commit")

	outcome := w.run(t, sync.Request{Worktrees: []string{solo}, NoFetch: true})

	require.Equal(t, domain.StatusDone, itemsOf(outcome)[solo].Status, itemsOf(outcome)[solo].Err)
	assert.FileExists(t, filepath.Join(w.at(solo), "mistake.txt"))
	assert.Equal(t, 2, rebaseOf(outcome.Plan, solo).Commits, "the commit moved and its own")
}

func TestEachBranchFastForwardedFirstIsAnItemBeforeTheWorktrees(t *testing.T) {
	w := newWorld(t)
	w.pushedOnOrigin(t, "main")

	outcome := w.run(t, naming(solo))

	assert.Equal(t, []string{"main", solo}, subjectsOf(outcome))
	forwarded := itemsOf(outcome)["main"]
	require.Equal(t, domain.StatusDone, forwarded.Status, forwarded.Err)
	assert.Equal(t, 1, forwarded.Detail.Commits, "the commits it took from origin")
	assert.Empty(t, forwarded.Detail.Parent)
}

func TestABranchFastForwardRefusedAtApplySkipsItsItemAndItsChildRebasesOntoItAsItStands(t *testing.T) {
	w := newWorld(t)
	gittest.Git(t, w.main, "branch", "topic", "main")
	gittest.Git(t, w.main, "push", "-q", "origin", "topic")
	w.pushedOnOrigin(t, "topic")
	gittest.Git(t, w.main, "fetch", "-q", "origin")
	w.recordParent(t, solo, "topic")
	planned, _ := w.plan(t, sync.Request{Worktrees: []string{solo}, NoFetch: true})
	// topic moves since the plan: its fast-forward to the commit planned is refused
	moved := gittest.Output(t, w.main, "commit-tree", "topic^{tree}", "-p", "topic", "-m", "local since")
	gittest.Git(t, w.main, "update-ref", "refs/heads/topic", moved)

	result := w.usecase.Apply(t.Context(), planned, (&operationtest.Recorder{}).Add)

	require.Len(t, result.Items, 2)
	refused := result.Items[0]
	assert.Equal(t, "topic", refused.Subject)
	require.Equal(t, domain.StatusSkipped, refused.Status, "a refused fast-forward never fails the run")
	assert.Equal(t, sync.CodeFFFailed, refused.Err.Code)
	assert.Equal(t, solo, refused.Err.Params["children"])
	assert.Equal(t, domain.StatusDone, result.Items[1].Status, result.Items[1].Err)
	assert.Nil(t, result.Failure(), "every worktree synced: the run succeeded")
}

func TestABranchToFastForwardIsInterruptedByACancelBeforeIt(t *testing.T) {
	w := newWorld(t)
	w.pushedOnOrigin(t, "main")
	planned, _ := w.plan(t, naming(solo))
	main := w.tip(t, "main")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	result := w.usecase.Apply(ctx, planned, (&operationtest.Recorder{}).Add)

	require.NotEmpty(t, result.Items)
	assert.Equal(t, "main", result.Items[0].Subject)
	assert.Equal(t, domain.ReasonInterrupted, result.Items[0].Reason)
	assert.Equal(t, main, w.tip(t, "main"))
}

func TestARefusedFastForwardBesideAConflictLeavesTheConflictTheRunsFailure(t *testing.T) {
	result := sync.Result{Items: []sync.Item{
		{Subject: "main", Status: domain.StatusSkipped, Err: &domain.Error{Kind: domain.KindConflict, Code: sync.CodeFFFailed}},
		{Subject: featB, Status: domain.StatusFailed, Err: &domain.Error{Kind: domain.KindConflict, Code: sync.CodeConflict}},
	}}

	require.NotNil(t, result.Failure())
	assert.Equal(t, sync.CodeConflict, result.Failure().Code)
}
