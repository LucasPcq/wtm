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

func TestAFailingQueryKeepsThePullRequestsTheOthersFound(t *testing.T) {
	gh, _ := graphQLGH(t, "b150")

	prs, err := github.GitHub{Dir: t.TempDir(), Binary: gh}.PullRequestsOf(t.Context(), branchesNamed(250))

	require.ErrorIs(t, err, worktree.ErrPullRequestsUnavailable)
	assert.ErrorContains(t, err, "gh api graphql")
	assert.Len(t, prs, 150)
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

func TestANodeGitHubNulledIsNoPullRequest(t *testing.T) {
	gh := partialGH(t, `{"data": {"repository": {"b0": {"nodes": [null, {"number": 7, "url": "u", "state": "CLOSED", "headRefName": "feat/a", "isCrossRepository": false}]}}},
		"errors": [{"type": "FORBIDDEN", "path": ["repository", "b0", "nodes", 0, "commits"], "message": "Resource not accessible by integration"}]}`)

	prs, err := github.GitHub{Dir: t.TempDir(), Binary: gh}.PullRequestsOf(t.Context(), []string{"feat/a"})

	require.ErrorIs(t, err, worktree.ErrPullRequestsUnavailable)
	assert.Equal(t, []worktree.PullRequest{{Number: 7, URL: "u", Branch: "feat/a", State: worktree.PullRequestClosed}}, prs)
}

func TestAFailureInEveryQueryKeepsWhatEachResolved(t *testing.T) {
	gh := partialGH(t, `{"data": {"repository": {"b0": {"nodes": [{"number": 1, "url": "u", "state": "MERGED", "headRefName": "feat/a", "isCrossRepository": false}]}}},
		"errors": [{"type": "FORBIDDEN", "message": "Resource not accessible by integration"}]}`)

	prs, err := github.GitHub{Dir: t.TempDir(), Binary: gh}.PullRequestsOf(t.Context(), branchesNamed(150))

	require.ErrorIs(t, err, worktree.ErrPullRequestsUnavailable)
	assert.Len(t, prs, 2)
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

func TestTheOpenPullRequestsAreListedWithWhatAPickerShowsOfThem(t *testing.T) {
	gh, log := loggedGH(t, `[{"number": 42, "url": "u42", "state": "OPEN", "headRefName": "feat/a", "headRefOid": "abc", "baseRefName": "main",
		"isCrossRepository": false, "title": "Add a", "isDraft": true, "author": {"login": "alice"}},
		{"number": 43, "url": "u43", "state": "OPEN", "headRefName": "main", "baseRefName": "main", "isCrossRepository": true, "author": {"login": "bob"}}]`)

	prs, err := github.GitHub{Dir: t.TempDir(), Binary: gh}.OpenPullRequests(t.Context(), worktree.AllPullRequests)

	require.NoError(t, err)
	assert.Equal(t, []worktree.PullRequest{
		{Number: 42, URL: "u42", Branch: "feat/a", Head: "abc", State: worktree.PullRequestOpen, Title: "Add a", Draft: true, Base: "main", Author: "alice"},
		{Number: 43, URL: "u43", Branch: "main", State: worktree.PullRequestOpen, Base: "main", Author: "bob", Fork: true},
	}, prs, "a fork's is listed, marked")
	assert.Equal(t, "pr list --state open --limit 50 --json number,url,state,headRefName,headRefOid,baseRefName,isCrossRepository,title,isDraft,author\n", readLog(t, log))
}

func TestTheOpenPullRequestsAreNarrowedToMineOrThoseAskingMyReview(t *testing.T) {
	tests := []struct {
		name   string
		filter worktree.PullRequestFilter
		want   string
	}{
		{"mine", worktree.MyPullRequests, " --author @me\n"},
		{"review", worktree.ReviewRequested, " --search review-requested:@me\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gh, log := loggedGH(t, `[]`)

			prs, err := github.GitHub{Dir: t.TempDir(), Binary: gh}.OpenPullRequests(t.Context(), tt.filter)

			require.NoError(t, err)
			assert.Empty(t, prs)
			assert.True(t, strings.HasSuffix(readLog(t, log), tt.want), readLog(t, log))
		})
	}
}

func TestAPullRequestIsReadByItsNumberWhateverItsState(t *testing.T) {
	gh, log := loggedGH(t, `{"number": 7, "url": "u7", "state": "MERGED", "headRefName": "feat/a", "headRefOid": "abc", "baseRefName": "release", "author": {"login": "alice"}}`)

	pr, err := github.GitHub{Dir: t.TempDir(), Binary: gh}.PullRequest(t.Context(), 7)

	require.NoError(t, err)
	assert.Equal(t, worktree.PullRequest{Number: 7, URL: "u7", Branch: "feat/a", Head: "abc", State: worktree.PullRequestMerged, Base: "release", Author: "alice"}, pr)
	assert.True(t, strings.HasPrefix(readLog(t, log), "pr view 7 --json "), readLog(t, log))
}

func TestANumberTheRepositoryHasNoPullRequestOfIsNotFound(t *testing.T) {
	gh := refusingGH(t, "GraphQL: Could not resolve to a PullRequest with the number of 99. (repository.pullRequest)")

	_, err := github.GitHub{Dir: t.TempDir(), Binary: gh}.PullRequest(t.Context(), 99)

	require.ErrorIs(t, err, worktree.ErrPullRequestNotFound)
	assert.NotErrorIs(t, err, worktree.ErrPullRequestsUnavailable)
	assert.EqualError(t, err, "pull request #99: pull request not found")
}

func TestAPullRequestGHCannotReadSaysWhy(t *testing.T) {
	signedOut := fakeGH(t, 1, "{}")
	noForge := refusingGH(t, "none of the git remotes configured for this repository point to a known GitHub host")

	_, signedOutErr := github.GitHub{Dir: t.TempDir(), Binary: signedOut}.PullRequest(t.Context(), 1)
	_, listErr := github.GitHub{Dir: t.TempDir(), Binary: signedOut}.OpenPullRequests(t.Context(), worktree.AllPullRequests)
	_, noForgeErr := github.GitHub{Dir: t.TempDir(), Binary: noForge}.PullRequest(t.Context(), 1)

	assert.EqualError(t, signedOutErr, "pull requests unavailable: run `gh auth login`")
	assert.EqualError(t, listErr, "pull requests unavailable: run `gh auth login`")
	assert.EqualError(t, noForgeErr, "pull requests unavailable: origin is not a GitHub repository")
}
