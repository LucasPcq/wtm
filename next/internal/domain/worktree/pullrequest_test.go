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

func TestAMergedPullRequestCleansItsWorktreeOnlyAtTheCommitItMergedAndNeverTheMainOneNorTheBase(t *testing.T) {
	repo := Repository{Snapshot: Snapshot{Branches: Branches{Local: []Branch{{Name: "main", Tip: "m1"}, {Name: "develop", Tip: "d1"}, {Name: "feat/a", Tip: "a1"}}}}}
	repo.Config.Worktrees.BaseBranch = "develop"
	merged := func(head string) PullRequest { return PullRequest{State: PullRequestMerged, Head: head} }
	onA := Worktree{State: OnBranch{Branch: "feat/a"}}
	cases := map[string]struct {
		pr     PullRequest
		w      Worktree
		cleans bool
	}{
		"merged":      {pr: merged("a1"), w: onA, cleans: true},
		"moved_on":    {pr: merged("a0"), w: onA},
		"no_head":     {pr: merged(""), w: onA},
		"open":        {pr: PullRequest{State: PullRequestOpen, Head: "a1"}, w: onA},
		"closed":      {pr: PullRequest{State: PullRequestClosed, Head: "a1"}, w: onA},
		"main":        {pr: merged("m1"), w: Worktree{Main: true, State: OnBranch{Branch: "main"}}},
		"base_branch": {pr: merged("d1"), w: Worktree{State: OnBranch{Branch: "develop"}}},
		"no_branch":   {pr: merged("a1"), w: Worktree{State: Detached{}}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, c.cleans, repo.Cleans(c.pr, c.w))
		})
	}
}
