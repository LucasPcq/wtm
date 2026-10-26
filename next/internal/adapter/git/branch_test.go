package git_test

import (
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"

	"github.com/LucasPcq/wtm/next/internal/adapter/git"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBranchesSplitsLocalFromRemoteAndDropsTheRemoteHead(t *testing.T) {
	repo, _ := newRepoWithOrigin(t)
	gittest.CreateBranch(t, repo.Dir, "feature")
	gittest.PushBranch(t, repo.Dir, "feature")
	gittest.CreateBranch(t, repo.Dir, "local-only")
	gittest.Git(t, repo.Dir, "remote", "set-head", "origin", "main")
	gittest.Git(t, repo.Dir, "update-ref", "refs/remotes/fork/elsewhere", "main")
	gittest.Git(t, repo.Dir, "update-ref", "refs/remotes/origin-fork/elsewhere", "main")

	branches, err := repo.Branches(t.Context())

	require.NoError(t, err)
	main := gittest.Output(t, repo.Dir, "rev-parse", "main")
	assert.Equal(t, worktree.Branches{
		Local:  []worktree.Branch{{Name: "feature", Tip: main}, {Name: "local-only", Tip: main}, {Name: "main", Tip: main}},
		Remote: []worktree.Branch{{Name: "feature", Tip: main}, {Name: "main", Tip: main}},
	}, branches)
}

func TestAheadBehindAgainstTheRemoteBranch(t *testing.T) {
	repo, origin := newRepoWithOrigin(t)
	commitFile(t, repo.Dir, "local", "1")
	advanceOnOrigin(t, origin, "main")
	require.NoError(t, repo.FetchRef(t.Context(), "main"))

	counts, err := repo.AheadBehind(t.Context(), "main")

	require.NoError(t, err)
	assert.Equal(t, worktree.AheadBehind{Ahead: 1, Behind: 1}, counts)
}

func TestAheadBehindFailsOnAnUnknownRefRatherThanCountZero(t *testing.T) {
	repo := newRepo(t)

	_, err := repo.AheadBehind(t.Context(), "main")

	var gitErr *git.Error
	require.ErrorAs(t, err, &gitErr)
}

func TestDeleteBranchDropsABranchEvenUnmerged(t *testing.T) {
	repo := newRepo(t)
	gittest.Git(t, repo.Dir, "checkout", "-b", "unmerged")
	commitFile(t, repo.Dir, "work", "1")
	gittest.Git(t, repo.Dir, "checkout", "main")

	require.NoError(t, repo.DeleteBranch(t.Context(), "unmerged"))

	assert.False(t, has(t, repo, worktree.LocalBranch{Name: "unmerged"}))
}

func TestTheFakeAndGitAgreeOnTheTipsAndCountsOfTheirBranches(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			main := worktree.LocalBranch{Name: "main"}

			assert.True(t, has(t, impl.port, worktree.LocalBranch{Name: "feature"}))
			assert.False(t, has(t, impl.port, worktree.LocalBranch{Name: "remote-only"}))
			assert.True(t, has(t, impl.port, worktree.RemoteBranch{Name: "remote-only"}))
			assert.False(t, has(t, impl.port, worktree.RemoteBranch{Name: "pushed"}))
			assert.Equal(t, worktree.AheadBehind{}, aheadBehind(t, impl.port, "main"))
			assert.Equal(t, worktree.AheadBehind{Ahead: 1}, aheadBehind(t, impl.port, "diverged"))
			assert.Equal(t, tip(t, impl.port, main), tip(t, impl.port, worktree.RemoteBranch{Name: "feature"}))
			assert.NotEqual(t, tip(t, impl.port, main), tip(t, impl.port, worktree.LocalBranch{Name: "unmerged"}))
			assert.NotEqual(t, tip(t, impl.port, worktree.LocalBranch{Name: "diverged"}), tip(t, impl.port, worktree.RemoteBranch{Name: "diverged"}))
		})
	}
}

func TestTheFakeAndGitDateTheTipOfEveryLocalBranch(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			branches, err := impl.port.Branches(t.Context())
			require.NoError(t, err)

			dates, err := impl.port.CommitDates(t.Context())

			require.NoError(t, err)
			require.Len(t, dates, len(branches.Local))
			for _, b := range branches.Local {
				assert.False(t, dates[b.Name].IsZero(), b.Name)
				assert.Equal(t, time.UTC, dates[b.Name].Location(), b.Name)
			}
		})
	}
}

func TestTheFakeAndGitCountBothWaysBetweenABranchAndItsBase(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			ahead, aheadErr := impl.port.AheadBehindOf(t.Context(), worktree.Span{Branch: "unmerged", Base: "main"})
			behind, behindErr := impl.port.AheadBehindOf(t.Context(), worktree.Span{Branch: "main", Base: "unmerged"})
			merged, mergedErr := impl.port.AheadBehindOf(t.Context(), worktree.Span{Branch: "feature", Base: "main"})
			_, missingErr := impl.port.AheadBehindOf(t.Context(), worktree.Span{Branch: "unmerged", Base: "nope"})

			require.NoError(t, aheadErr)
			assert.Equal(t, worktree.AheadBehind{Ahead: 1}, ahead)
			require.NoError(t, behindErr)
			assert.Equal(t, worktree.AheadBehind{Behind: 1}, behind)
			require.NoError(t, mergedErr)
			assert.Zero(t, merged)
			require.Error(t, missingErr)
		})
	}
}

func TestTheFakeAndGitTellWhetherACommitIsAnotherOrOneOfItsAncestors(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			main, unmerged := tip(t, impl.port, worktree.LocalBranch{Name: "main"}), tip(t, impl.port, worktree.LocalBranch{Name: "unmerged"})
			unknown := "1234567890123456789012345678901234567890"

			behind, behindErr := impl.port.IsAncestor(t.Context(), worktree.Ancestry{Ancestor: main, Descendant: unmerged})
			ahead, aheadErr := impl.port.IsAncestor(t.Context(), worktree.Ancestry{Ancestor: unmerged, Descendant: main})
			same, sameErr := impl.port.IsAncestor(t.Context(), worktree.Ancestry{Ancestor: main, Descendant: main})
			_, unknownErr := impl.port.IsAncestor(t.Context(), worktree.Ancestry{Ancestor: main, Descendant: unknown})

			require.NoError(t, behindErr)
			assert.True(t, behind)
			require.NoError(t, aheadErr)
			assert.False(t, ahead)
			require.NoError(t, sameErr)
			assert.True(t, same, "a commit is its own ancestor")
			require.Error(t, unknownErr, "a head never fetched is no commit of the repository")
		})
	}
}

func TestTheFakeAndGitListTheLastCommitsOfAWorktreeNewestFirst(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			commits, err := impl.port.Commits(t.Context(), impl.at("linked"), 5)
			one, oneErr := impl.port.Commits(t.Context(), impl.at("linked"), 1)

			require.NoError(t, err)
			var subjects []string
			for _, commit := range commits {
				subjects = append(subjects, commit.Subject)
			}
			assert.Equal(t, []string{"linked work", gittest.Root}, subjects)
			require.NoError(t, oneErr)
			assert.Len(t, one, 1)
		})
	}
}

func TestTheFakeAndGitCountWhatDeletingABranchWouldLose(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			lost := map[string]int{}
			for _, name := range []string{"main", "feature", "unmerged", "linked", "diverged"} {
				count, err := impl.port.Unpushed(t.Context(), worktree.Losing{Branch: name})
				require.NoError(t, err, name)
				lost[name] = count
			}

			assert.Equal(t, map[string]int{"main": 0, "feature": 0, "unmerged": 1, "linked": 1, "diverged": 1}, lost)
		})
	}
}

func TestTheFakeAndGitCountWhatRemovingAWorktreeLosesFromTheCommitItStandsOn(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			worktrees, err := impl.port.Worktrees(t.Context())
			require.NoError(t, err)

			lost := map[string]int{}
			for _, w := range worktrees {
				name := filepath.Base(w.Path)
				if !slices.Contains([]string{"linked", "detached", "rebasing", "gone"}, name) {
					continue
				}
				count, err := impl.port.Unpushed(t.Context(), worktree.Losing{Head: w.Head, Going: []string{name}})
				require.NoError(t, err, name)
				lost[name] = count
			}
			both, err := impl.port.Unpushed(t.Context(), worktree.Losing{Branch: "linked", Head: worktrees[slices.IndexFunc(worktrees, func(w worktree.Worktree) bool { return w.Branch() == "linked" })].Head})
			require.NoError(t, err)

			assert.Equal(t, map[string]int{"linked": 1, "detached": 0, "rebasing": 0, "gone": 0}, lost, "the commit a worktree stands on counts what its branch holds once the branch goes too, its directory gone or not")
			assert.Equal(t, 1, both, "a commit the branch and HEAD both hold is counted once")
		})
	}
}

func TestTheFakeAndGitCountNothingForATipThatNamesNoCommit(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			count, err := impl.port.Unpushed(t.Context(), worktree.Losing{Branch: "remote-only"})

			require.NoError(t, err)
			assert.Equal(t, 0, count)
		})
	}
}

func TestTheFakeAndGitCountTheCommitsHeldOnlyByTheBranchesGoingTogether(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			require.NoError(t, impl.port.AddWorktree(t.Context(), worktree.Addition{Path: filepath.Join(impl.root, "topic"), Checkout: worktree.NewBranch{Name: "topic", From: worktree.LocalBranch{Name: "unmerged"}}}))

			alone, err := impl.port.Unpushed(t.Context(), worktree.Losing{Branch: "unmerged"})
			require.NoError(t, err)
			together, err := impl.port.Unpushed(t.Context(), worktree.Losing{Branch: "unmerged", Going: []string{"topic"}})
			require.NoError(t, err)

			assert.Equal(t, 0, alone)
			assert.Equal(t, 1, together)
		})
	}
}

func TestTheFakeAndGitRefuseTheSameBranchChanges(t *testing.T) {
	refusals := []struct {
		name string
		call func(t *testing.T, impl implementation) error
	}{
		{name: "delete_checked_out_branch", call: func(t *testing.T, impl implementation) error {
			return impl.port.DeleteBranch(t.Context(), "main")
		}},
		{name: "delete_rebasing_branch", call: func(t *testing.T, impl implementation) error {
			return impl.port.DeleteBranch(t.Context(), "rebasing")
		}},
		{name: "delete_missing_branch", call: func(t *testing.T, impl implementation) error {
			return impl.port.DeleteBranch(t.Context(), "nope")
		}},
		{name: "count_unknown_ref", call: func(t *testing.T, impl implementation) error {
			_, err := impl.port.AheadBehind(t.Context(), "unmerged")
			return err
		}},
	}
	for _, impl := range implementations(t) {
		for _, refusal := range refusals {
			t.Run(impl.name+"_"+refusal.name, func(t *testing.T) {
				before := gittest.StateOf(t, impl.port)

				err := refusal.call(t, impl)

				require.Error(t, err)
				assert.Equal(t, before, gittest.StateOf(t, impl.port))
			})
		}
	}
}
