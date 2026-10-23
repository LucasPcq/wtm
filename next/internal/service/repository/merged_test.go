package repository_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestABranchIsBehindItsMergedPullRequestOnlyWhenGitSaysItsTipIsAnAncestorOfItsHead(t *testing.T) {
	git := gittest.NewFake(gittest.FakeParams{
		Main: "/repo/main", Branch: "main", Remote: []string{"feat/behind"},
		Worktrees: []worktree.Worktree{linkedAt("feat/behind"), linkedAt("feat/at"), linkedAt("feat/apart"), linkedAt("feat/unknown")},
		Commits: map[worktree.Ref][]string{
			worktree.LocalBranch{Name: "feat/behind"}:  {"b1"},
			worktree.RemoteBranch{Name: "feat/behind"}: {"b1", "b2"},
			worktree.LocalBranch{Name: "feat/at"}:      {"a1"},
			worktree.LocalBranch{Name: "feat/apart"}:   {"p1"},
		},
	})
	snapshot, err := git.Snapshot(t.Context())
	require.NoError(t, err)
	repo := worktree.Repository{Snapshot: snapshot}
	merged := func(branch, head string) worktree.PullRequest {
		return worktree.PullRequest{Branch: branch, Head: head, State: worktree.PullRequestMerged}
	}
	prs := []worktree.PullRequest{merged("feat/behind", "b2"), merged("feat/at", "a1"), merged("feat/apart", gittest.Root), merged("feat/unknown", "never fetched")}

	behind := repository.Behind(t.Context(), git, repo, prs, snapshot.Checkouts())

	assert.Equal(t, map[string]bool{"feat/behind": true}, behind)
	assert.NotContains(t, git.Calls(), "IsAncestor a1", "a tip at its head needs no git call")
}
