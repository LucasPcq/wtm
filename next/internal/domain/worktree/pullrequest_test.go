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

func TestAMergedPullRequestCleansItsWorktreeAtOrBehindItsHeadAndNeverTheMainOneNorTheBase(t *testing.T) {
	repo := Repository{Snapshot: Snapshot{Branches: Branches{Local: []Branch{{Name: "main", Tip: "m1"}, {Name: "develop", Tip: "d1"}, {Name: "feat/a", Tip: "a1"}}}}}
	repo.Config.Worktrees.BaseBranch = "develop"
	merged := func(head string) PullRequest { return PullRequest{State: PullRequestMerged, Head: head} }
	onA := Worktree{State: OnBranch{Branch: "feat/a"}}
	cases := map[string]struct {
		pr     PullRequest
		w      Worktree
		behind bool
		cleans bool
	}{
		"at_its_head":     {pr: merged("a1"), w: onA, cleans: true},
		"behind_its_head": {pr: merged("a2"), w: onA, behind: true, cleans: true},
		"moved_on":        {pr: merged("a0"), w: onA},
		"no_head":         {pr: merged(""), w: onA, behind: true},
		"open":            {pr: PullRequest{State: PullRequestOpen, Head: "a1"}, w: onA},
		"closed":          {pr: PullRequest{State: PullRequestClosed, Head: "a1"}, w: onA},
		"main":            {pr: merged("m1"), w: Worktree{Main: true, State: OnBranch{Branch: "main"}}},
		"base_branch":     {pr: merged("d1"), w: Worktree{State: OnBranch{Branch: "develop"}}},
		"no_branch":       {pr: merged("a1"), w: Worktree{State: Detached{}}, behind: true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, c.cleans, repo.Cleans(c.pr, c.w, c.behind))
		})
	}
}

func TestAClosedPullRequestFinishesItsWorktreeAsAMergedOneDoes(t *testing.T) {
	repo := Repository{Snapshot: Snapshot{Branches: Branches{Local: []Branch{{Name: "main", Tip: "m1"}, {Name: "feat/a", Tip: "a1"}}}}}
	repo.Config.Worktrees.BaseBranch = "main"
	onA := Worktree{State: OnBranch{Branch: "feat/a"}}
	cases := map[string]struct {
		pr       PullRequest
		w        Worktree
		behind   bool
		finished bool
	}{
		"merged":        {pr: PullRequest{State: PullRequestMerged, Head: "a1"}, w: onA, finished: true},
		"closed":        {pr: PullRequest{State: PullRequestClosed, Head: "a1"}, w: onA, finished: true},
		"closed_behind": {pr: PullRequest{State: PullRequestClosed, Head: "a2"}, w: onA, behind: true, finished: true},
		"closed_moved":  {pr: PullRequest{State: PullRequestClosed, Head: "a0"}, w: onA},
		"open":          {pr: PullRequest{State: PullRequestOpen, Head: "a1"}, w: onA},
		"none":          {w: onA},
		"base_branch":   {pr: PullRequest{State: PullRequestClosed, Head: "m1"}, w: Worktree{State: OnBranch{Branch: "main"}}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, c.finished, repo.Finished(c.pr, c.w, c.behind))
		})
	}
}

func TestOnlyAFinishedPullRequestPastItsBranchTipAsksWhetherTheTipIsBehind(t *testing.T) {
	repo := Repository{Snapshot: Snapshot{Branches: Branches{Local: []Branch{{Name: "main", Tip: "m1"}, {Name: "feat/a", Tip: "a1"}}}}}
	repo.Config.Worktrees.BaseBranch = "main"
	onA := Worktree{State: OnBranch{Branch: "feat/a"}}

	ancestry, past := repo.FinishedPast(PullRequest{State: PullRequestMerged, Head: "a2"}, onA)
	_, closed := repo.FinishedPast(PullRequest{State: PullRequestClosed, Head: "a2"}, onA)
	_, atHead := repo.FinishedPast(PullRequest{State: PullRequestMerged, Head: "a1"}, onA)
	_, open := repo.FinishedPast(PullRequest{State: PullRequestOpen, Head: "a2"}, onA)
	_, base := repo.FinishedPast(PullRequest{State: PullRequestMerged, Head: "m2"}, Worktree{State: OnBranch{Branch: "main"}})

	assert.True(t, past)
	assert.Equal(t, Ancestry{Ancestor: "a1", Descendant: "a2"}, ancestry)
	assert.True(t, closed)
	assert.False(t, atHead, "a tip at the head needs no git call")
	assert.False(t, open)
	assert.False(t, base)
}

func TestOnlyAnOpenPullRequestHasAReviewInProgress(t *testing.T) {
	pr := PullRequest{Title: "Add a", Draft: true, Checks: Checks{Passed: 1}, Decision: "APPROVED"}

	for _, state := range []PullRequestState{PullRequestMerged, PullRequestClosed} {
		pr.State = state
		assert.Nil(t, pr.Progress(), state)
	}
	pr.State = PullRequestOpen
	assert.Equal(t, &Progress{Title: "Add a", Draft: true, Checks: Checks{Passed: 1}, Decision: "APPROVED"}, pr.Progress())
}
