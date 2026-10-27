package repository

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// Behind returns, by branch, whether the tip of each of worktrees' branches is behind the head of its merged or closed pull request among prs, the fact worktree.Repository.Finished and Cleans take. Git is asked only of a branch whose pull request moved past its tip (worktree.Repository.FinishedPast); one git cannot tell of, its head never fetched included, is not behind.
func Behind(ctx context.Context, git Ancestors, repo worktree.Repository, prs []worktree.PullRequest, worktrees []worktree.Worktree) map[string]bool {
	behind := map[string]bool{}
	for _, w := range worktrees {
		pr, _ := worktree.PullRequestOf(prs, w.Branch())
		ancestry, ask := repo.FinishedPast(pr, w)
		if !ask {
			continue
		}
		if is, err := git.IsAncestor(ctx, ancestry); err == nil && is {
			behind[w.Branch()] = true
		}
	}

	return behind
}
