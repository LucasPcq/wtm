package prune_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/command/prune"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPruneRemovesEveryWorktreeDoneWithThatIsSafe(t *testing.T) {
	w := newWorld(t, seed{})

	outcome, err := w.usecase.Run(t.Context(), prune.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	require.NotNil(t, outcome.Result)
	assert.Equal(t, map[string]domain.Status{merged: domain.StatusDone, closed: domain.StatusDone, gone: domain.StatusDone, goneWork: domain.StatusSkipped, dirty: domain.StatusSkipped}, statuses(outcome.Result.Items))
	assert.Equal(t, domain.ReasonRuledOut, itemOf(t, outcome.Result.Items, dirty).Reason, "the command chose it: it is left as it is")
	for _, name := range []string{merged, closed, gone} {
		assert.True(t, w.gone(t, name), name)
	}
	for _, name := range []string{child, moved, goneWork, dirty, open, plain} {
		assert.True(t, w.stands(t, name), name)
	}
	assert.Contains(t, codes(outcome.Warnings), prune.CodeKept, "a worktree kept is still named once the others went")
}

func TestEachItemSaysWhyItsWorktreeWasDoneWith(t *testing.T) {
	w := newWorld(t, seed{})

	result, _ := w.apply(t, prune.Request{})

	assert.Equal(t, &prune.Detail{Path: w.at(merged), Reason: prune.ReasonMerged}, itemOf(t, result.Items, merged).Detail)
	assert.Equal(t, &prune.Detail{Path: w.at(gone), Reason: prune.ReasonGone}, itemOf(t, result.Items, gone).Detail)
}

// Legacy took a merged or closed pull request for done whatever the branch did since: a commit made after the merge went with it.
func TestABranchMovedPastItsPullRequestIsNotDoneWith(t *testing.T) {
	w := newWorld(t, seed{})

	planned, _ := w.plan(t, prune.Request{})
	_, err := w.usecase.Run(t.Context(), pruning(moved), domain.RunOptions{DryRun: true})

	assert.NotContains(t, namesOf(planned.Worktrees), moved)
	refused := failureOf(t, err)
	assert.Equal(t, domain.KindInvalid, refused.Kind)
	assert.Equal(t, map[string][]string{"worktree[0]": {prune.CodeNotFinished}}, fieldCodes(refused.Fields))
	assert.Equal(t, "it has commits past its pull request's head", refused.Fields[0].Params["why"])
}

// B2: legacy counted a branch whose upstream origin deleted as safe, and deleted it with git branch -D, the commits only it held with it.
func TestAGoneBranchHoldingCommitsNoOtherBranchHoldsIsKept(t *testing.T) {
	w := newWorld(t, seed{})

	outcome, err := w.usecase.Run(t.Context(), prune.Request{Gone: true}, domain.RunOptions{})

	require.NoError(t, err)
	assert.Equal(t, []string{goneWork}, keptOf(outcome.Plan))
	assert.Equal(t, []worktree.Blocker{worktree.BlockerUnpushed}, itemOfPlan(t, outcome.Plan, goneWork).Blockers)
	assert.True(t, w.stands(t, goneWork))
	assert.True(t, w.gone(t, gone))
}

// B5: legacy said only "Nothing to prune" when every match was unsafe, the unsafe ones and their --force lost.
func TestEachUnsafeWorktreeDoneWithIsKeptWithAWarningAndTheForcedRequest(t *testing.T) {
	w := newWorld(t, seed{})

	planned, warnings := w.plan(t, prune.Request{})

	assert.ElementsMatch(t, []string{goneWork, dirty}, keptOf(planned))
	assert.Empty(t, itemOfPlan(t, planned, dirty).Phases, "a worktree left as it is runs no phase")
	assert.NotEmpty(t, itemOfPlan(t, planned, dirty).Blockers)
	assert.Equal(t, []string{prune.CodeKept, prune.CodeKept}, codes(warnings))
	assert.Nil(t, warnings[0].FollowUp)
	forced := warnings[1].FollowUp
	require.NotNil(t, forced)
	assert.Equal(t, prune.CodeForce, forced.Code)
	assert.Contains(t, forced.Answers, domain.Answered{Path: domain.PathWorktree, Values: keptOf(planned)})
	assert.Contains(t, forced.Answers, domain.Answered{Path: prune.PathForce, Values: []string{"true"}})
}

// The web plans with no_fetch, and the request that prunes the kept ones anyway, typed in a terminal, did not read origin.
func TestTheRequestThatPrunesTheKeptOnesAnywayReadsOriginAfresh(t *testing.T) {
	w := newWorld(t, seed{})

	_, warnings := w.plan(t, prune.Request{NoFetch: true})

	forced := warnings[len(warnings)-1].FollowUp
	require.NotNil(t, forced)
	for _, answered := range forced.Answers {
		assert.NotEqual(t, prune.PathNoFetch, answered.Path)
	}
}

func TestForceWithNoWorktreeNamedPrunesTheUnsafeOnesToo(t *testing.T) {
	w := newWorld(t, seed{})

	result, _ := w.apply(t, prune.Request{Force: true})

	assert.Len(t, result.Items, 5)
	assert.True(t, w.gone(t, dirty))
	assert.True(t, w.gone(t, goneWork))
}

func TestAnUnsafeWorktreeNamedRefusesTheRequestUntilForced(t *testing.T) {
	w := newWorld(t, seed{})

	_, err := w.usecase.Run(t.Context(), pruning(merged, dirty), domain.RunOptions{})
	forced, forcedErr := w.usecase.Run(t.Context(), prune.Request{Worktrees: []string{dirty}, Force: true}, domain.RunOptions{})

	refused := failureOf(t, err)
	assert.Equal(t, domain.KindRefused, refused.Kind)
	assert.Equal(t, prune.CodeUnsafe, refused.Code)
	assert.Equal(t, map[string][]string{"worktree[1]": {"prune.dirty"}}, fieldCodes(refused.Fields))
	require.NotNil(t, refused.FollowUp)
	assert.Equal(t, prune.CodeForce, refused.FollowUp.Code)
	assert.True(t, w.stands(t, merged), "nothing is removed before the refusal")
	require.NoError(t, forcedErr)
	assert.Equal(t, map[string]domain.Status{dirty: domain.StatusDone}, statuses(forced.Result.Items))
}

func TestAWorktreeNamedThatIsNotDoneWithIsAProblemOfItsEntry(t *testing.T) {
	w := newWorld(t, seed{})

	_, err := w.usecase.Run(t.Context(), pruning(open, plain, "main", "feat/nope", merged, merged), domain.RunOptions{DryRun: true})

	refused := failureOf(t, err)
	assert.Equal(t, map[string][]string{
		"worktree[0]": {prune.CodeNotFinished},
		"worktree[1]": {prune.CodeNotFinished},
		"worktree[2]": {prune.CodeMainWorktree},
		"worktree[3]": {prune.CodeNotFinished},
		"worktree[5]": {domain.CodeDistinct},
	}, fieldCodes(refused.Fields))
	assert.Equal(t, "its pull request is still open", refused.Fields[0].Params["why"])
}

func TestTheReasonsAskedNarrowWhatIsDoneWith(t *testing.T) {
	w := newWorld(t, seed{})

	closedOnly, _ := w.plan(t, prune.Request{Closed: true})
	_, err := w.usecase.Run(t.Context(), prune.Request{Worktrees: []string{merged}, Gone: true}, domain.RunOptions{DryRun: true})

	assert.Equal(t, []string{closed}, namesOf(closedOnly.Worktrees))
	assert.Equal(t, []prune.Reason{prune.ReasonClosed}, closedOnly.Reasons)
	assert.Equal(t, "its pull request is merged, which this run does not ask for", failureOf(t, err).Fields[0].Params["why"])
}

// B1: legacy's dry run said it reparented the children, and the run then left them orphaned.
func TestTheChildrenLeftBehindAreRecappedAsTheRunWillLeaveThem(t *testing.T) {
	w := newWorld(t, seed{})

	orphaning, _ := w.plan(t, pruning(merged))
	reparenting, _ := w.plan(t, prune.Request{Worktrees: []string{merged}, ReparentChildren: true})
	result, _ := w.apply(t, pruning(merged))

	assert.Equal(t, "feat/child (left on feat/merged)", orphaning.Recap[1].Values[0].Text)
	assert.Equal(t, "feat/child → main", reparenting.Recap[1].Values[0].Text)
	assert.Equal(t, []string{prune.CodeOrphaned}, codes(result.Warnings))
	record, err := w.meta.Load(child)
	require.NoError(t, err)
	assert.Equal(t, merged, record.SourceBranch)
}

func TestReparentingMovesEachChildOntoTheNearestWorktreeThatStays(t *testing.T) {
	w := newWorld(t, seed{})

	result, report := w.apply(t, prune.Request{Worktrees: []string{merged}, ReparentChildren: true})

	assert.Empty(t, result.Warnings)
	record, err := w.meta.Load(child)
	require.NoError(t, err)
	assert.Equal(t, "main", record.SourceBranch)
	updated := eventsOf[domain.WorktreeUpdated](report.All())
	require.Len(t, updated, 1)
	assert.Equal(t, child, updated[0].Worktree.Branch)
	assert.Equal(t, w.at(child), updated[0].Worktree.Path)
}

// B4: legacy stopped at the first failure, and the worktrees after it were in no list of its report.
func TestAFailingHookKeepsItsWorktreeStopsNoOtherAndEachHasItsItem(t *testing.T) {
	w := newWorld(t, seed{Config: withHook(`[ "{{branch}}" != "feat/merged" ]`)})

	result, _ := w.apply(t, prune.Request{})

	assert.Equal(t, map[string]domain.Status{merged: domain.StatusFailed, closed: domain.StatusDone, gone: domain.StatusDone, goneWork: domain.StatusSkipped, dirty: domain.StatusSkipped}, statuses(result.Items))
	require.NotNil(t, itemOf(t, result.Items, merged).Err)
	assert.Equal(t, prune.CodeHookFailed, itemOf(t, result.Items, merged).Err.Code)
	assert.True(t, w.stands(t, merged))
	assert.Empty(t, result.Warnings, "the child of a parent that stayed is not orphaned")
}

// B6: legacy ran the hooks in a directory that was gone, failed with /bin/sh's error and removed nothing.
func TestAWorktreeWhoseDirectoryIsGoneIsPrunedWithoutItsHooks(t *testing.T) {
	w := newWorld(t, seed{Config: withHook("exit 3")})
	require.NoError(t, os.RemoveAll(w.at(merged)))

	result, _ := w.apply(t, pruning(merged))

	assert.Equal(t, domain.StatusDone, result.Items[0].Status)
	assert.Equal(t, []string{prune.CodeHooksSkipped}, codes(result.Items[0].Warnings))
	assert.True(t, w.gone(t, merged))
}

// B8: legacy told the user only --gone applies even when they asked for it alone, skipped the open pull request check without a word, and its JSON carried no warning.
func TestAForgeThatCannotBeAskedIsWarnedOfByWhatItLeavesUndecided(t *testing.T) {
	w := newWorld(t, seed{PRErr: worktree.ErrPullRequestsUnavailable})

	all, allWarnings := w.plan(t, prune.Request{})
	_, goneWarnings := w.plan(t, prune.Request{Gone: true})

	assert.Equal(t, []string{gone}, prunedOf(all), "merged and closed match nothing")
	assert.Equal(t, prune.Name+domain.PRsUnavailable, allWarnings[0].Code)
	assert.Contains(t, allWarnings[0].Message, "merged and closed ones match nothing")
	assert.NotContains(t, goneWarnings[0].Message, "merged and closed")
	assert.Contains(t, goneWarnings[0].Message, "open ones aren't checked")
}

// B9: legacy fetched origin at its best and swallowed the failure: offline, gone branches were read from stale refs without a word.
func TestAFetchThatFailsIsWarnedOf(t *testing.T) {
	w := newWorld(t, seed{})
	w.usecase.Git = offline{w.repo}

	planned, warnings := w.plan(t, prune.Request{Gone: true})

	assert.Empty(t, planned.Worktrees, "the branches origin deleted still read as fetched")
	require.NotEmpty(t, warnings)
	assert.Equal(t, prune.CodeFetchFailed, warnings[0].Code)
}

func TestNoFetchReadsOriginAsLastFetchedAndOnlyGoneFetches(t *testing.T) {
	w := newWorld(t, seed{})
	fetched := false
	w.usecase.Git = noFetch{w.repo, &fetched}

	w.plan(t, prune.Request{Gone: true, NoFetch: true})
	notGone := fetched
	w.plan(t, prune.Request{Merged: true})
	notAsked := fetched
	w.plan(t, prune.Request{Gone: true})

	assert.False(t, notGone)
	assert.False(t, notAsked, "only gone branches need origin fetched")
	assert.True(t, fetched)
}

func TestJobsUpMakeAWorktreeUnsafeAndTheJobsTheDaemonCannotTellAreWarnedOf(t *testing.T) {
	w := newWorld(t, seed{Jobs: []run.Job{{Name: "web", WorkDir: "", Up: true}}})
	w.usecase.Jobs = func(context.Context) ([]run.Job, error) {
		return []run.Job{{Name: "web", WorkDir: w.at(merged), Up: true}}, nil
	}

	planned, _ := w.plan(t, prune.Request{})
	w.usecase.Jobs = func(context.Context) ([]run.Job, error) { return nil, run.ErrUnknown }
	_, warnings := w.plan(t, prune.Request{Merged: true})

	assert.Contains(t, keptOf(planned), merged)
	assert.Contains(t, codes(warnings), prune.Name+domain.JobsUnknown)
}

func TestNothingDoneWithIsAnEmptyPlanAndAnEmptyApply(t *testing.T) {
	w := newWorld(t, seed{})
	w.usecase.PullRequests = func(context.Context, []string) ([]worktree.PullRequest, error) { return nil, nil }

	outcome, err := w.usecase.Run(t.Context(), prune.Request{Merged: true}, domain.RunOptions{})

	require.NoError(t, err)
	assert.Empty(t, outcome.Plan.Worktrees)
	assert.Empty(t, outcome.Plan.Recap)
	assert.Empty(t, outcome.Result.Items)
	assert.Nil(t, outcome.Result.Failure())
}

func TestACancelSkipsTheWorktreesNotStarted(t *testing.T) {
	w := newWorld(t, seed{})
	ctx, cancel := context.WithCancel(t.Context())
	planned, _ := w.plan(t, prune.Request{})
	cancel()

	result := w.usecase.Apply(ctx, planned, func(domain.Event) {})

	assert.Equal(t, map[string]domain.Status{merged: domain.StatusSkipped, closed: domain.StatusSkipped, gone: domain.StatusSkipped, goneWork: domain.StatusSkipped, dirty: domain.StatusSkipped}, statuses(result.Items))
	assert.Equal(t, domain.ReasonInterrupted, itemOf(t, result.Items, merged).Reason)
	assert.True(t, w.stands(t, merged))
}

func TestAPruneUnderTheCallersFeetEndsOnTheMainWorktree(t *testing.T) {
	w := newWorld(t, seed{})
	req := pruning(merged).From(domain.Caller{Cwd: filepath.Join(w.at(merged))})

	outcome, err := w.usecase.Run(t.Context(), req, domain.RunOptions{})

	require.NoError(t, err)
	require.Len(t, outcome.Result.FollowUps, 1)
	assert.Equal(t, prune.CodeGo, outcome.Result.FollowUps[0].Code)
}

// A git failure while reading the facts was taken for no fact at all: "nothing to do", exit 0.
func TestAGitFailureWhileReadingRefusesTheRunRatherThanFindingNothing(t *testing.T) {
	w := newWorld(t, seed{})
	w.usecase.Git = broken{w.repo}

	_, err := w.usecase.Run(t.Context(), prune.Request{}, domain.RunOptions{})

	require.ErrorIs(t, err, errBroken)
	assert.True(t, w.stands(t, merged))
}

func TestWhyANamedWorktreeIsNotDoneWithClaimsOnlyWhatTheRunRead(t *testing.T) {
	w := newWorld(t, seed{})
	offline := newWorld(t, seed{PRErr: worktree.ErrPullRequestsUnavailable})

	_, mergedOnly := w.usecase.Run(t.Context(), prune.Request{Worktrees: []string{plain}, Merged: true}, domain.RunOptions{DryRun: true})
	_, every := w.usecase.Run(t.Context(), pruning(plain), domain.RunOptions{DryRun: true})
	_, unasked := offline.usecase.Run(t.Context(), pruning(plain), domain.RunOptions{DryRun: true})

	assert.Equal(t, "it has no merged or closed pull request", failureOf(t, mergedOnly).Fields[0].Params["why"])
	assert.Equal(t, "it has no merged or closed pull request, and origin has not deleted its branch", failureOf(t, every).Fields[0].Params["why"])
	assert.Equal(t, "its pull requests could not be asked, and origin has not deleted its branch", failureOf(t, unasked).Fields[0].Params["why"])
}

func TestAGitFailureWhileCheckingTheCandidatesRefusesTheRun(t *testing.T) {
	w := newWorld(t, seed{})
	w.usecase.Teardown.Git = uncountable{w.repo}

	_, err := w.usecase.Run(t.Context(), prune.Request{}, domain.RunOptions{})

	require.ErrorIs(t, err, errBroken)
	assert.True(t, w.stands(t, merged))
}

// Every candidate counted as going: feat/copy, named alone, was refused for a commit feat/gone-work, which stays, holds.
func TestAWorktreeNamedIsCheckedAgainstTheWorktreesThatStay(t *testing.T) {
	w := newWorld(t, seed{})
	gittest.Git(t, w.main, "branch", "feat/copy", goneWork)
	gittest.Git(t, w.main, "worktree", "add", "-q", w.at("feat/copy"), "feat/copy")
	gittest.Git(t, w.main, "config", "branch.feat/copy.remote", "origin")
	gittest.Git(t, w.main, "config", "branch.feat/copy.merge", "refs/heads/feat/copy")

	outcome, err := w.usecase.Run(t.Context(), prune.Request{Worktrees: []string{"feat/copy"}, Gone: true}, domain.RunOptions{})

	require.NoError(t, err)
	assert.Equal(t, map[string]domain.Status{"feat/copy": domain.StatusDone}, statuses(outcome.Result.Items))
	assert.True(t, w.stands(t, goneWork))
}

func TestAWorktreeDoneWithWhoseRecordANewerWtmWroteRefusesThePrune(t *testing.T) {
	w := newWorld(t, seed{})
	w.rawRecord(t, merged, newerRecord)

	for _, req := range []prune.Request{{}, {Force: true}, pruning(merged)} {
		_, err := w.usecase.Run(t.Context(), req, domain.RunOptions{})

		refused := failureOf(t, err)
		assert.Equal(t, prune.CodeSchemaNewer, refused.Code)
		assert.Contains(t, refused.Message, merged)
		var newer *domain.SchemaNewer
		assert.ErrorAs(t, err, &newer)
	}
	assert.DirExists(t, w.at(merged))
	assert.True(t, w.stands(t, gone))
}

func TestAWorktreeDoneWithWhoseRecordDoesNotDecodeIsPrunedAllTheSame(t *testing.T) {
	w := newWorld(t, seed{})
	w.rawRecord(t, merged, "{not json")

	_, err := w.usecase.Run(t.Context(), pruning(merged), domain.RunOptions{})

	require.NoError(t, err)
	assert.True(t, w.gone(t, merged))
}
