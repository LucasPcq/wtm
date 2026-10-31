package checkout_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/git"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/command/checkout"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestABranchOnlyOriginHasIsCreatedTrackingIt(t *testing.T) {
	w := newWorld(t)

	result, _ := w.apply(t, checkout.Request{PR: prRemote})

	require.Len(t, result.Items, 1)
	assert.Equal(t, domain.StatusDone, result.Items[0].Status)
	assert.Equal(t, "feat/remote", gittest.Output(t, w.path("feat/remote"), "branch", "--show-current"))
	assert.Equal(t, "refs/heads/feat/remote", gittest.Output(t, w.main, "config", "branch.feat/remote.merge"))
	meta := worktree.Meta{SourceBranch: "main", CreatedAt: "2026-10-16T12:00:00Z", EnvStrategy: project.EnvMain, Isolation: project.Isolated}
	assert.Equal(t, &checkout.Detail{Path: w.path("feat/remote"), Metadata: meta, Standing: worktree.Standing{Upstream: "origin/feat/remote"}}, result.Items[0].Detail)
	assert.Equal(t, mainEnv, readFile(t, filepath.Join(w.path("feat/remote"), ".env")))
}

func TestALocalBranchIsCheckedOutAsItIs(t *testing.T) {
	w := newWorld(t)
	tip := gittest.Output(t, w.main, "rev-parse", "feat/diverged")

	planned, warnings := w.plan(t, checkout.Request{PR: prDiverged})
	result, _ := w.apply(t, checkout.Request{PR: prDiverged})

	assert.Equal(t, checkout.ActionCheckOut, planned.Action)
	assert.Equal(t, []string{checkout.CodeSourceDiverged}, codes(warnings))
	assert.Equal(t, tip, gittest.Output(t, w.path("feat/diverged"), "rev-parse", "HEAD"), "its own commit kept")
	assert.Equal(t, worktree.Standing{ExistingBranch: true, OriginState: worktree.Diverged, OriginAhead: 1, OriginBehind: 1}, result.Items[0].Detail.Standing)
}

// B11: legacy said a branch kept behind origin in a plain note, a diverged one in a warning.
func TestABranchKeptBehindOriginIsAWarningOfItsItem(t *testing.T) {
	w := newWorld(t)

	result, _ := w.apply(t, checkout.Request{PR: prBehind})

	assert.Equal(t, []string{checkout.CodeBehindOrigin}, codes(result.Items[0].Warnings))
}

func TestABranchFastForwardedFirstStandsAsOriginHasIt(t *testing.T) {
	w := newWorld(t)
	req := checkout.Request{PR: prBehind}
	req.FastForward = new(true)

	planned, _ := w.plan(t, req)
	result, _ := w.apply(t, req)

	assert.Equal(t, []string{"feat/behind"}, planned.FastForward)
	assert.Equal(t, gittest.Output(t, w.main, "rev-parse", "origin/feat/behind"), gittest.Output(t, w.path("feat/behind"), "rev-parse", "HEAD"))
	assert.Empty(t, result.Items[0].Warnings)
	assert.Equal(t, worktree.UpToDate, result.Items[0].Detail.OriginState)
}

// Q5: the base only origin has is the parent, spelled as create's sources spell it.
func TestTheParentOnlyOriginHasIsRecordedAsOriginsBranch(t *testing.T) {
	w := newWorld(t)

	result, _ := w.apply(t, checkout.Request{PR: prOnBase})

	assert.Equal(t, "origin/release", result.Items[0].Detail.Metadata.SourceBranch)
}

// B5, Q3: legacy never read the state of the pull request it checked out.
func TestAPullRequestNoLongerOpenIsCheckedOutWithAWarning(t *testing.T) {
	w := newWorld(t)

	planned, warnings := w.plan(t, checkout.Request{PR: prMerged})

	assert.Equal(t, worktree.PullRequestMerged, planned.PullRequest.State)
	assert.Equal(t, []string{checkout.CodePRNotOpen}, codes(warnings))
	assert.Equal(t, "pull request #10 is merged", warnings[0].Message)
}

func TestAFetchThatFailsIsAWarningAndThePlanStandsOnOriginAsLastFetched(t *testing.T) {
	w := newWorld(t)
	w.usecase.Git = unfetched{git.Repo{Dir: w.main}}

	planned, warnings := w.plan(t, checkout.Request{PR: prRemote})

	assert.Equal(t, checkout.ActionResume, planned.Action)
	assert.Equal(t, []string{checkout.CodeFetchFailed}, codes(warnings))
}

func TestAForkOrAPullRequestNotFoundIsNeitherFetchedNorPlanned(t *testing.T) {
	w := newWorld(t)
	w.usecase.Git = unfetched{git.Repo{Dir: w.main}}

	fork := w.facts(t, checkout.Request{PR: prFork})
	unknown := w.facts(t, checkout.Request{PR: prNone})

	assert.NoError(t, fork.FetchErr)
	assert.Nil(t, unknown.PR)
	assert.NoError(t, unknown.FetchErr)
}

func TestARunHoldsTheRepositoryThenTheBranchOfThePullRequest(t *testing.T) {
	w := newWorld(t)

	planned, _ := w.plan(t, checkout.Request{PR: prRemote})

	assert.Equal(t, []domain.Lock{{Scope: domain.ScopeRepo, Mode: domain.LockShared}}, checkout.Request{PR: prRemote}.Locks())
	assert.Equal(t, []domain.Lock{{Scope: domain.ScopeRepo, Mode: domain.LockShared}, {Scope: domain.ScopeWorktree, Name: "feat/remote", Mode: domain.LockExclusive}}, planned.Locks())
}

func TestARunCreatesTheWorktreeAndEndsOnTheStepIntoIt(t *testing.T) {
	w := newWorld(t)

	outcome, err := w.usecase.Run(t.Context(), checkout.Request{PR: prRemote}, domain.RunOptions{})

	require.NoError(t, err)
	require.NotNil(t, outcome.Result)
	assert.Equal(t, []domain.FollowUp{{Command: domain.CommandGo, Answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{"feat/remote"}}}, Code: checkout.CodeGo, Message: "jump in"}}, outcome.Result.FollowUps)
	assert.DirExists(t, w.path("feat/remote"))
}

func TestAHookThatFailsLeavesTheWorktreeAndItsStepIntoIt(t *testing.T) {
	w := newWorld(t)
	writeFile(t, filepath.Join(w.stateDir, "config.toml"), withHook("exit 3"))

	result, _ := w.apply(t, checkout.Request{PR: prRemote})

	assert.Equal(t, domain.StatusFailed, result.Items[0].Status)
	require.NotNil(t, result.Items[0].Detail)
	assert.DirExists(t, w.path("feat/remote"))
	assert.Len(t, result.FollowUps, 1)
}

func TestTheAnswersAskedToBeRememberedAreKeptOnceTheWorktreeIsCreated(t *testing.T) {
	w := newWorld(t)
	req := checkout.Request{PR: prRemote, Given: provision.Given{EnvFrom: project.EnvExample, Remember: []string{domain.PathEnvFrom}}}

	result, _ := w.apply(t, req)

	assert.Equal(t, []string{domain.PathEnvFrom}, result.Remembered)
	assert.Contains(t, readFile(t, filepath.Join(w.stateDir, "config.toml")), `env_strategy = "example"`)
}

func TestThePlanRecapsThePullRequestItsBranchAndItsParent(t *testing.T) {
	w := newWorld(t)

	planned, _ := w.plan(t, checkout.Request{PR: prRemote})

	var texts []string
	for _, row := range planned.Recap {
		texts = append(texts, row.Label+": "+row.Values[0].Text)
	}
	assert.Equal(t, "Pull request: #1 PR feat/remote|Branch: feat/remote (resumes origin/feat/remote)|Parent: main|Env: main", strings.Join(texts, "|"))
}

func TestAPullRequestGitHubCannotBeAskedOfRefusesThePlan(t *testing.T) {
	w := newWorld(t)
	w.usecase.PullRequest = github{asked: w.asked, err: worktree.ErrPullRequestsUnavailable}.pullRequest

	_, _, err := w.usecase.Plan(t.Context(), checkout.Request{PR: prRemote})

	assert.Equal(t, checkout.CodeGitHubUnavailable, failureOf(t, err).Code)
	assert.Equal(t, domain.KindPrecondition, failureOf(t, err).Kind)
}

func TestNoPullRequestUnattendedIsRefusedWithoutAskingGitHub(t *testing.T) {
	w := newWorld(t)

	_, _, err := w.usecase.Plan(t.Context(), checkout.Request{})

	assert.Equal(t, map[string]string{checkout.PathPR: domain.CodeRequired}, problems(err))
	assert.Empty(t, *w.asked)
}

func TestARepositoryWithoutWtmIsRefusedBeforeGitHubIsAsked(t *testing.T) {
	w := newWorld(t)
	require.NoError(t, os.Remove(filepath.Join(w.stateDir, "config.toml")))

	_, _, err := w.usecase.Plan(t.Context(), checkout.Request{PR: prRemote})

	assert.Equal(t, checkout.CodeConfigNotFound, failureOf(t, err).Code)
	assert.Empty(t, *w.asked)
}

// Review round 1: a merged pull request's branch, auto-deleted on origin after a fetch without --prune, was resumed from its stale remote-tracking branch.
func TestABranchOriginDeletedSinceTheLastFetchIsNotTrackedButRefused(t *testing.T) {
	w := newWorld(t)
	req := checkout.Request{PR: prDeleted}

	_, err := checkout.Resolve(w.facts(t, req), req)

	assert.Equal(t, map[string]string{checkout.PathPR: checkout.CodeHeadMissing}, problems(err))
}

func TestALocalBranchOriginDeletedIsCheckedOutAsItIsWithNothingToFastForward(t *testing.T) {
	w := newWorld(t)

	questions := w.questions(t, checkout.Request{PR: prStale})
	planned, warnings := w.plan(t, checkout.Request{PR: prStale})

	assert.Equal(t, domain.SkipNotBehind, questions.FastForward.Skip, "its stale remote-tracking branch says behind; origin has no such branch")
	assert.Equal(t, checkout.ActionCheckOut, planned.Action)
	assert.Empty(t, warnings, "no fetch_failed: the fetch said what it had to")
}

func TestAnEditOfThePullRequestsTitleLeavesThePreviewStanding(t *testing.T) {
	w := newWorld(t)
	preview, err := w.usecase.Run(t.Context(), checkout.Request{PR: prRemote}, domain.RunOptions{DryRun: true})
	require.NoError(t, err)
	prs := pullRequests()
	prs[0].Title, prs[0].Draft = "Renamed", true
	w.usecase.PullRequest = github{prs: prs, asked: w.asked}.pullRequest

	outcome, err := w.usecase.Run(t.Context(), checkout.Request{PR: prRemote}, domain.RunOptions{ExpectFingerprint: preview.Fingerprint})

	require.NoError(t, err)
	assert.Equal(t, domain.StatusDone, outcome.Result.Items[0].Status)
}

func TestANeighbourWhoseRecordANewerWtmWroteRefusesTheRunBeforeAnythingIsCreated(t *testing.T) {
	w := newWorld(t)
	writeFile(t, filepath.Join(w.stateDir, "run.toml"), jobsToml)
	dir, ok := worktree.MetaDir(worktree.MetaDirParams{StateDir: w.stateDir, Branch: "feat/held"})
	require.True(t, ok)
	writeFile(t, filepath.Join(dir, "meta.json"), `{"schema": 99, "ordinal": 1}`)
	planned, _ := checkout.Decide(w.facts(t, checkout.Request{PR: prRemote}), checkout.Request{PR: prRemote, From: "main"})
	require.Contains(t, planned.Phases, domain.PhaseOrdinal, "the plan numbers the worktree")

	_, err := w.usecase.Run(t.Context(), checkout.Request{PR: prRemote}, domain.RunOptions{})

	refused := failureOf(t, err)
	assert.Equal(t, checkout.CodeMetaNewer, refused.Code)
	assert.Equal(t, domain.KindPrecondition, refused.Kind)
	var newer *domain.SchemaNewer
	assert.ErrorAs(t, err, &newer)
	assert.NoDirExists(t, w.path("feat/remote"))
	assert.Empty(t, gittest.Output(t, w.main, "branch", "--list", "feat/remote"))
}
