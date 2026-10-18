package github_test

import (
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/github"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPullRequestsAreTheOpenOnesGHLists(t *testing.T) {
	gh := fakeGH(t, 0, `[{"number": 12, "headRefName": "feat/a", "url": "https://github.com/acme/acme/pull/12", "state": "OPEN"},
		{"number": 13, "headRefName": "main", "url": "https://github.com/acme/acme/pull/13", "state": "OPEN", "isCrossRepository": true}]`)

	prs, err := github.GitHub{Dir: t.TempDir(), Binary: gh}.PullRequests(t.Context())

	require.NoError(t, err)
	assert.Equal(t, worktree.PullRequests{Open: []worktree.PullRequest{{Number: 12, URL: "https://github.com/acme/acme/pull/12", Branch: "feat/a"}}}, prs, "a fork's pull request is not this repository's branch")
}

func TestPullRequestsPastTheCapSaySo(t *testing.T) {
	gh := fakeGH(t, 0, listed(51))

	prs, err := github.GitHub{Dir: t.TempDir(), Binary: gh}.PullRequests(t.Context())

	require.NoError(t, err)
	assert.True(t, prs.Capped)
	assert.Len(t, prs.Open, 50)
}

func TestPullRequestsSayWhyGHCannotBeAsked(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "gh")
	signedOut := fakeGH(t, 1, "[]")

	_, missingErr := github.GitHub{Dir: t.TempDir(), Binary: missing}.PullRequests(t.Context())
	_, signedOutErr := github.GitHub{Dir: t.TempDir(), Binary: signedOut}.PullRequests(t.Context())

	require.ErrorIs(t, missingErr, worktree.ErrPullRequestsUnavailable)
	assert.EqualError(t, missingErr, "pull requests unavailable: install gh, GitHub's CLI")
	require.ErrorIs(t, signedOutErr, worktree.ErrPullRequestsUnavailable)
	assert.EqualError(t, signedOutErr, "pull requests unavailable: run `gh auth login`")
}

func TestPullRequestsRefuseOutputThatIsNotTheirList(t *testing.T) {
	gh := fakeGH(t, 0, "not json")

	_, err := github.GitHub{Dir: t.TempDir(), Binary: gh}.PullRequests(t.Context())

	require.ErrorIs(t, err, worktree.ErrPullRequestsUnavailable)
}
