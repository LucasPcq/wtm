package github_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/github"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPullRequestCountsTheChecksOfItsLastCommitByOutcomeAndCarriesItsReview(t *testing.T) {
	gh := fakeGH(t, 0, `{"data": {"repository": {"b0": {"nodes": [{"number": 12, "url": "u", "state": "OPEN", "headRefName": "feat/a", "headRefOid": "abc123",
		"title": "Add a", "isDraft": true, "reviewDecision": "APPROVED",
		"commits": {"nodes": [{"commit": {"statusCheckRollup": {"contexts": {
			"checkRunCountsByState": [{"state": "SUCCESS", "count": 2}, {"state": "NEUTRAL", "count": 1}, {"state": "SKIPPED", "count": 1}, {"state": "FAILURE", "count": 1},
				{"state": "ACTION_REQUIRED", "count": 1}, {"state": "IN_PROGRESS", "count": 1}, {"state": "QUEUED", "count": 1}, {"state": "SOMETHING_NEW", "count": 1}],
			"statusContextCountsByState": [{"state": "SUCCESS", "count": 1}, {"state": "PENDING", "count": 1}, {"state": "EXPECTED", "count": 1}, {"state": "ERROR", "count": 1}]
		}}}}]}}]}}}}`)

	prs, err := github.GitHub{Dir: t.TempDir(), Binary: gh}.PullRequestsOf(t.Context(), []string{"feat/a"})

	require.NoError(t, err)
	assert.Equal(t, []worktree.PullRequest{{
		Number: 12, URL: "u", Branch: "feat/a", Head: "abc123", State: worktree.PullRequestOpen, Title: "Add a", Draft: true, Decision: "APPROVED",
		Checks: worktree.Checks{Passed: 5, Failed: 3, Pending: 5},
	}}, prs, "an unknown state fails closed, an action required is pending")
}

func TestAPullRequestWithoutChecksCountsNone(t *testing.T) {
	gh := fakeGH(t, 0, `{"data": {"repository": {"b0": {"nodes": [{"number": 12, "url": "u", "state": "OPEN", "headRefName": "feat/a",
		"commits": {"nodes": [{"commit": {"statusCheckRollup": null}}]}}]}}}}`)

	prs, err := github.GitHub{Dir: t.TempDir(), Binary: gh}.PullRequestsOf(t.Context(), []string{"feat/a"})

	require.NoError(t, err)
	assert.Equal(t, worktree.Checks{}, prs[0].Checks)
}

func TestTheNewestPullRequestOfEachBranchIsReadWhateverItsState(t *testing.T) {
	gh := fakeGH(t, 0, `{"data": {"repository": {
		"b0": {"nodes": [{"number": 14, "url": "u14", "state": "MERGED", "headRefName": "feat/a", "isCrossRepository": false}]},
		"b1": {"nodes": [{"number": 21, "url": "u21", "state": "OPEN", "headRefName": "main", "isCrossRepository": true}, {"number": 20, "url": "u20", "state": "CLOSED", "headRefName": "main", "isCrossRepository": false}]},
		"b2": {"nodes": [{"number": 30, "url": "u30", "state": "OPEN", "headRefName": "feat/fork", "isCrossRepository": true}]},
		"b3": {"nodes": []}}}}`)

	prs, err := github.GitHub{Dir: t.TempDir(), Binary: gh}.PullRequestsOf(t.Context(), []string{"feat/a", "main", "feat/fork", "feat/none"})

	require.NoError(t, err)
	assert.Equal(t, []worktree.PullRequest{
		{Number: 14, URL: "u14", Branch: "feat/a", State: worktree.PullRequestMerged},
		{Number: 20, URL: "u20", Branch: "main", State: worktree.PullRequestClosed},
	}, prs, "a fork's pull request is not this repository's branch")
}

func TestAnOpenPullRequestIsPreferredToANewerOneNoLongerOpen(t *testing.T) {
	gh := fakeGH(t, 0, `{"data": {"repository": {"b0": {"nodes": [
		{"number": 31, "url": "u31", "state": "OPEN", "headRefName": "feat/a", "isCrossRepository": true},
		{"number": 30, "url": "u30", "state": "CLOSED", "headRefName": "feat/a", "isCrossRepository": false},
		{"number": 29, "url": "u29", "state": "OPEN", "headRefName": "feat/a", "isCrossRepository": false}]}}}}`)

	prs, err := github.GitHub{Dir: t.TempDir(), Binary: gh}.PullRequestsOf(t.Context(), []string{"feat/a"})

	require.NoError(t, err)
	require.Len(t, prs, 1)
	assert.Equal(t, 29, prs[0].Number, "a fork's open one is not this branch's")
}

func TestABranchlessWorktreeIsNotAskedOfAndNoneLeftAsksNothing(t *testing.T) {
	gh, log := graphQLGH(t, "")
	signedOut := fakeGH(t, 1, "{}")
	branches := []string{"", "feat/a", ""}

	prs, err := github.GitHub{Dir: t.TempDir(), Binary: gh}.PullRequestsOf(t.Context(), branches)
	none, noneErr := github.GitHub{Dir: t.TempDir(), Binary: signedOut}.PullRequestsOf(t.Context(), []string{""})

	require.NoError(t, err)
	assert.Len(t, prs, 1)
	calls, err := os.ReadFile(log)
	require.NoError(t, err)
	assert.Equal(t, "feat/a", strings.TrimSpace(string(calls)))
	assert.Equal(t, []string{"", "feat/a", ""}, branches, "the caller's branches are left as they are")
	require.NoError(t, noneErr, "gh is not run, so it is not found signed out")
	assert.Nil(t, none)
}

func TestThePullRequestsOfManyBranchesAreAskedAHundredAtATime(t *testing.T) {
	gh, log := graphQLGH(t, "")

	prs, err := github.GitHub{Dir: t.TempDir(), Binary: gh}.PullRequestsOf(t.Context(), branchesNamed(250))

	require.NoError(t, err)
	assert.Len(t, prs, 250)
	calls, err := os.ReadFile(log)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(calls)), "\n")
	require.Len(t, lines, 3)
	assert.Len(t, strings.Fields(lines[0]), 100)
	assert.Len(t, strings.Fields(lines[2]), 50)
}

func TestAFailingQueryKeepsThePullRequestsFoundBeforeIt(t *testing.T) {
	gh, _ := graphQLGH(t, "b150")

	prs, err := github.GitHub{Dir: t.TempDir(), Binary: gh}.PullRequestsOf(t.Context(), branchesNamed(250))

	require.ErrorIs(t, err, worktree.ErrPullRequestsUnavailable)
	assert.ErrorContains(t, err, "gh api graphql")
	assert.Len(t, prs, 100)
}

func TestAQueryGitHubResolvesInPartKeepsWhatItResolved(t *testing.T) {
	gh := partialGH(t, `{"data": {"repository": {"b0": {"nodes": [{"number": 9, "url": "u", "state": "MERGED", "headRefName": "feat/a", "headRefOid": "abc123",
		"isCrossRepository": false, "commits": {"nodes": [{"commit": {"statusCheckRollup": null}}]}}]}}},
		"errors": [{"type": "FORBIDDEN", "path": ["repository", "b0", "nodes", 0, "commits"], "message": "Resource not accessible by integration"}]}`)

	prs, err := github.GitHub{Dir: t.TempDir(), Binary: gh}.PullRequestsOf(t.Context(), []string{"feat/a"})

	require.ErrorIs(t, err, worktree.ErrPullRequestsUnavailable)
	assert.ErrorContains(t, err, "Resource not accessible by integration")
	assert.Equal(t, []worktree.PullRequest{{Number: 9, URL: "u", Branch: "feat/a", Head: "abc123", State: worktree.PullRequestMerged}}, prs)
}

func TestAnInterruptedReadIsTheContextsErrorNotGitHubUnavailable(t *testing.T) {
	gh, _ := graphQLGH(t, "")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := github.GitHub{Dir: t.TempDir(), Binary: gh}.PullRequestsOf(ctx, []string{"feat/a"})

	require.ErrorIs(t, err, context.Canceled)
	assert.NotErrorIs(t, err, worktree.ErrPullRequestsUnavailable)
}

func TestThePullRequestsOfBranchesSayWhyGHCannotBeAsked(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "gh")
	signedOut := fakeGH(t, 1, "{}")
	garbled := fakeGH(t, 0, "not json")

	_, missingErr := github.GitHub{Dir: t.TempDir(), Binary: missing}.PullRequestsOf(t.Context(), []string{"feat/a"})
	_, signedOutErr := github.GitHub{Dir: t.TempDir(), Binary: signedOut}.PullRequestsOf(t.Context(), []string{"feat/a"})
	_, garbledErr := github.GitHub{Dir: t.TempDir(), Binary: garbled}.PullRequestsOf(t.Context(), []string{"feat/a"})

	require.ErrorIs(t, missingErr, worktree.ErrPullRequestsUnavailable)
	assert.EqualError(t, missingErr, "pull requests unavailable: install gh, GitHub's CLI")
	assert.EqualError(t, signedOutErr, "pull requests unavailable: run `gh auth login`")
	assert.ErrorIs(t, garbledErr, worktree.ErrPullRequestsUnavailable)
}
