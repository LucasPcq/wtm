package git_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/git"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheFakeAndGitRebaseABranchOntoAnotherFromWhereItForked(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			require.NoError(t, impl.port.Fetch(t.Context()))
			fork, err := impl.port.ForkPoint(t.Context(), worktree.Span{Branch: "linked", Base: "main"})
			require.NoError(t, err)
			require.Equal(t, 1, fork.Own)

			rebased, err := impl.port.Rebase(t.Context(), worktree.Rebase{Path: impl.at("linked"), Onto: worktree.RemoteBranch{Name: "main"}, Upstream: fork.Commit})

			require.NoError(t, err)
			assert.False(t, rebased.Conflicted())
			commits, err := impl.port.Commits(t.Context(), impl.at("linked"), 2)
			require.NoError(t, err)
			assert.Equal(t, []string{"linked work", "upstream main"}, []string{commits[0].Subject, commits[1].Subject})
		})
	}
}

func TestTheFakeAndGitPushABranchOverWhatOriginWasExpectedToHold(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			stale := impl.port.Push(t.Context(), worktree.Push{Branch: "unmerged", Expect: tip(t, impl.port, worktree.LocalBranch{Name: "main"})})
			pushed := impl.port.Push(t.Context(), worktree.Push{Branch: "linked"})
			again := impl.port.Push(t.Context(), worktree.Push{Branch: "linked", Expect: tip(t, impl.port, worktree.LocalBranch{Name: "main"})})

			require.Error(t, stale, "origin has no unmerged")
			require.NoError(t, pushed)
			require.NoError(t, again, "origin has it already")
			assert.Equal(t, tip(t, impl.port, worktree.LocalBranch{Name: "linked"}), tip(t, impl.port, worktree.RemoteBranch{Name: "linked"}))
		})
	}
}

func TestTheFakeAndGitCountTheCommitsOfOriginABranchNeverHeld(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			require.NoError(t, impl.port.Fetch(t.Context()))

			theirs, theirsErr := impl.port.NeverHeld(t.Context(), "diverged")
			held, heldErr := impl.port.NeverHeld(t.Context(), "feature")

			require.NoError(t, theirsErr)
			assert.Equal(t, 1, theirs)
			require.NoError(t, heldErr)
			assert.Equal(t, 1, held, "origin's feature moved on")
		})
	}
}

func TestARebaseThatConflictsIsAbortedAndTheWorktreeLeftAsItWas(t *testing.T) {
	repo, child := conflictingChild(t)
	before := head(t, child, "HEAD")

	rebased, err := repo.Rebase(t.Context(), worktree.Rebase{Path: child, Onto: worktree.LocalBranch{Name: "main"}, Upstream: forkOf(t, repo, "child", "main")})

	require.NoError(t, err)
	assert.Equal(t, []string{"shared.txt"}, rebased.Conflicts)
	assert.False(t, rebased.Kept)
	assert.Equal(t, before, head(t, child, "HEAD"))
	assert.Empty(t, gittest.Output(t, child, "status", "--porcelain"))
}

func TestARebaseThatConflictsIsLeftInProgressWhenAskedToKeepIt(t *testing.T) {
	repo, child := conflictingChild(t)

	rebased, err := repo.Rebase(t.Context(), worktree.Rebase{Path: child, Onto: worktree.LocalBranch{Name: "main"}, Upstream: forkOf(t, repo, "child", "main"), Keep: true})

	require.NoError(t, err)
	assert.Equal(t, []string{"shared.txt"}, rebased.Conflicts)
	assert.True(t, rebased.Kept)
	assert.DirExists(t, filepath.Join(gittest.Output(t, child, "rev-parse", "--absolute-git-dir"), "rebase-merge"))
}

func TestARebaseWithAutostashCarriesTheUncommittedChangesOver(t *testing.T) {
	repo, child := childOfMain(t)
	commitFile(t, repo.Dir, "main.txt", "moved on")
	require.NoError(t, os.WriteFile(filepath.Join(child, "child.txt"), []byte("work in progress\n"), 0o644))

	refused, refusedErr := repo.Rebase(t.Context(), worktree.Rebase{Path: child, Onto: worktree.LocalBranch{Name: "main"}, Upstream: forkOf(t, repo, "child", "main")})
	rebased, err := repo.Rebase(t.Context(), worktree.Rebase{Path: child, Onto: worktree.LocalBranch{Name: "main"}, Upstream: forkOf(t, repo, "child", "main"), Autostash: true})

	require.Error(t, refusedErr)
	assert.False(t, refused.Conflicted())
	require.NoError(t, err)
	assert.False(t, rebased.Stashed)
	assert.Equal(t, head(t, repo.Dir, "main"), head(t, child, "HEAD~1"))
	assert.Equal(t, "work in progress\n", contentOf(t, filepath.Join(child, "child.txt")))
}

// B4: the legacy sync rebased a child from its parent's tip, which replays the parent's old commits once the parent was rewritten by hand.
func TestAChildRebasedFromItsForkPointReplaysOnlyItsOwnCommitsOnceItsParentWasRewritten(t *testing.T) {
	repo, child := conflictingChild(t)
	grandchild := filepath.Join(gittest.TempDir(t), "grandchild")
	gittest.Git(t, repo.Dir, "worktree", "add", "-q", "-b", "grandchild", grandchild, "child")
	commitFile(t, grandchild, "grandchild.txt", "its own")
	resolveByHand(t, repo, child)

	rebased, err := repo.Rebase(t.Context(), worktree.Rebase{Path: grandchild, Onto: worktree.LocalBranch{Name: "child"}, Upstream: forkOf(t, repo, "grandchild", "child")})

	require.NoError(t, err)
	assert.False(t, rebased.Conflicted())
	assert.Equal(t, head(t, repo.Dir, "child"), head(t, grandchild, "HEAD~1"))
	fork, err := repo.ForkPoint(t.Context(), worktree.Span{Branch: "grandchild", Base: "child"})
	require.NoError(t, err)
	assert.Equal(t, 1, fork.Own, "its own commit alone, none of child's")
}

// B1: the legacy sync read a branch whose conflict was resolved by hand as diverged from origin for ever.
func TestABranchRebasedByHandSincePushedHeldEveryCommitOfOrigin(t *testing.T) {
	repo, child := conflictingChild(t)
	gittest.PushBranch(t, child, "child")
	resolveByHand(t, repo, child)

	never, err := repo.NeverHeld(t.Context(), "child")

	require.NoError(t, err)
	assert.Zero(t, never)
}

// B10: the legacy sync pushed with a bare lease, which a fetch made since the plan satisfies.
func TestAPushIsRefusedOnceOriginMovedPastTheCommitExpected(t *testing.T) {
	repo, origin := newRepoWithOrigin(t)
	planned := head(t, repo.Dir, "origin/main")
	advanceOnOrigin(t, origin, "main")
	require.NoError(t, repo.Fetch(t.Context()))
	commitFile(t, repo.Dir, "local.txt", "mine")

	err := repo.Push(t.Context(), worktree.Push{Branch: "main", Expect: planned})

	var gitErr *git.Error
	require.ErrorAs(t, err, &gitErr)
}
