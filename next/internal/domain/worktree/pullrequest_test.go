package worktree

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTheNewestPullRequestOfABranchIsFoundByItsName(t *testing.T) {
	prs := []PullRequest{{Number: 12, Branch: "feat/a"}, {Number: 9, Branch: "feat/b"}}

	found, ok := PullRequestOf(prs, "feat/b")
	_, none := PullRequestOf(prs, "feat/c")
	_, detached := PullRequestOf([]PullRequest{{Number: 1}}, "")

	assert.True(t, ok)
	assert.Equal(t, 9, found.Number)
	assert.False(t, none)
	assert.False(t, detached, "a worktree on no branch has no pull request")
}

func TestAMergedPullRequestCleansItsWorktreeUnlessItIsTheMainOneOrTheBase(t *testing.T) {
	merged := PullRequest{State: PullRequestMerged}
	cases := map[string]struct {
		pr     PullRequest
		w      Worktree
		cleans bool
	}{
		"merged":      {pr: merged, w: Worktree{State: OnBranch{Branch: "feat/a"}}, cleans: true},
		"open":        {pr: PullRequest{State: PullRequestOpen}, w: Worktree{State: OnBranch{Branch: "feat/a"}}},
		"closed":      {pr: PullRequest{State: PullRequestClosed}, w: Worktree{State: OnBranch{Branch: "feat/a"}}},
		"main":        {pr: merged, w: Worktree{Main: true, State: OnBranch{Branch: "main"}}},
		"base_branch": {pr: merged, w: Worktree{State: OnBranch{Branch: "develop"}}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, c.cleans, c.pr.Cleans(c.w, "develop"))
		})
	}
}
