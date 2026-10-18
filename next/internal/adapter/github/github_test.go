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
	gh := fakeGH(t, 0, `[{"number": 12, "headRefName": "feat/a", "url": "https://github.com/acme/acme/pull/12", "state": "OPEN"}]`)

	prs, err := github.GitHub{Dir: t.TempDir(), Binary: gh}.PullRequests(t.Context())

	require.NoError(t, err)
	assert.Equal(t, []worktree.PullRequest{{Number: 12, URL: "https://github.com/acme/acme/pull/12", State: "open", Branch: "feat/a"}}, prs)
}

func TestPullRequestsSayWhyGHCannotBeAsked(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "gh")
	signedOut := fakeGH(t, 1, "[]")

	_, missingErr := github.GitHub{Dir: t.TempDir(), Binary: missing}.PullRequests(t.Context())
	_, signedOutErr := github.GitHub{Dir: t.TempDir(), Binary: signedOut}.PullRequests(t.Context())

	require.ErrorIs(t, missingErr, worktree.ErrGitHubMissing)
	require.ErrorIs(t, signedOutErr, worktree.ErrGitHubSignedOut)
}

func TestPullRequestsRefuseOutputThatIsNotTheirList(t *testing.T) {
	gh := fakeGH(t, 0, "not json")

	_, err := github.GitHub{Dir: t.TempDir(), Binary: gh}.PullRequests(t.Context())

	require.Error(t, err)
}
