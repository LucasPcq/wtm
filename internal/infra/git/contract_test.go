package git_test

import (
	"context"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/infra/git"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

func TestTheFakeAndGitReadTheSameRepository(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			ctx := t.Context()
			at := impl.at

			main, mainErr := impl.port.MainWorktree(ctx)
			top, topErr := impl.port.Toplevel(ctx)
			common, commonErr := impl.port.GitCommonDir(ctx)

			require.NoError(t, mainErr)
			assert.Equal(t, at("main"), main.Path)
			require.NoError(t, topErr)
			assert.Equal(t, at("main"), top)
			require.NoError(t, commonErr)
			assert.Equal(t, filepath.Join(at("main"), ".git"), common)
			assert.Equal(t, gittest.State{
				Worktrees: []git.Worktree{
					{Path: at("main"), Main: true, State: git.OnBranch{Branch: "main"}},
					{Path: at("detached"), State: git.Detached{}},
					{Path: at("dirty"), State: git.OnBranch{Branch: "dirty"}},
					{Path: at("gone"), Prunable: true, State: git.OnBranch{Branch: "gone"}},
					{Path: at("linked"), State: git.OnBranch{Branch: "linked"}},
					{Path: at("locked"), Locked: true, State: git.OnBranch{Branch: "locked"}},
					{Path: at("rebasing"), State: git.Rebasing{Branch: "rebasing"}},
				},
				Branches: git.Branches{
					Local:  named("dirty", "diverged", "feature", "gone", "linked", "locked", "main", "rebasing", "unmerged"),
					Remote: named("diverged", "feature", "main", "remote-only"),
				},
			}, gittest.StateOf(t, impl.port))
		})
	}
}

func TestTheFakeAndGitAgreeOnWhatARefIs(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			ctx := t.Context()
			main := git.LocalBranch{Name: "main"}

			assert.True(t, refExists(t, impl.port, git.LocalBranch{Name: "feature"}))
			assert.False(t, refExists(t, impl.port, git.LocalBranch{Name: "remote-only"}))
			assert.True(t, refExists(t, impl.port, git.RemoteBranch{Name: "remote-only"}))
			assert.False(t, refExists(t, impl.port, git.RemoteBranch{Name: "pushed"}))
			assert.Equal(t, git.AheadBehind{}, aheadBehind(t, impl.port, git.AheadBehindParams{Of: main, Against: git.RemoteBranch{Name: "main"}}))
			assert.Equal(t, git.AheadBehind{Ahead: 1}, aheadBehind(t, impl.port, git.AheadBehindParams{Of: git.LocalBranch{Name: "unmerged"}, Against: main}))
			assert.Equal(t, git.AheadBehind{Ahead: 1, Behind: 1}, aheadBehind(t, impl.port, git.AheadBehindParams{Of: git.LocalBranch{Name: "linked"}, Against: git.LocalBranch{Name: "unmerged"}}))
			assert.Equal(t, tip(t, impl.port, main), tip(t, impl.port, git.RemoteBranch{Name: "feature"}))
			assert.NotEqual(t, tip(t, impl.port, main), tip(t, impl.port, git.LocalBranch{Name: "unmerged"}))
			_, err := impl.port.Tip(ctx, git.LocalBranch{Name: "nope"})
			require.Error(t, err)
		})
	}
}

func TestTheFakeAndGitListEachBranchWithItsTip(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			tips := tipsOf(t, impl.port)

			for ref, listed := range tips {
				assert.Equal(t, tip(t, impl.port, ref), listed, ref)
			}
			assert.Equal(t, tips[git.LocalBranch{Name: "main"}], tips[git.RemoteBranch{Name: "feature"}])
			assert.NotEqual(t, tips[git.LocalBranch{Name: "diverged"}], tips[git.RemoteBranch{Name: "diverged"}])
		})
	}
}

func TestTheFakeAndGitTellADirtyWorktreeFromACleanOne(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			dirty, dirtyErr := impl.port.Dirty(t.Context(), impl.at("dirty"))
			clean, cleanErr := impl.port.Dirty(t.Context(), impl.at("linked"))
			_, notErr := impl.port.Dirty(t.Context(), filepath.Join(impl.root, "taken", "nowhere"))

			require.NoError(t, dirtyErr)
			assert.True(t, dirty)
			require.NoError(t, cleanErr)
			assert.False(t, clean)
			require.Error(t, notErr)
		})
	}
}

func TestTheFakeAndGitLocateAPathAsGitWritesIt(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			cases := []struct {
				path string
				want git.Location
			}{
				{path: filepath.Join(impl.root, "main"), want: git.Location{Path: impl.at("main"), Exists: true}},
				{path: filepath.Join(impl.root, "taken"), want: git.Location{Path: impl.at("taken"), Exists: true}},
				{path: filepath.Join(impl.root, "gone"), want: git.Location{Path: impl.at("gone")}},
				{path: filepath.Join(impl.root, "new", "topic"), want: git.Location{Path: filepath.Join(impl.at("new"), "topic")}},
				{path: "../linked", want: git.Location{Path: impl.at("linked"), Exists: true}},
			}
			for _, c := range cases {
				location, err := impl.port.Locate(t.Context(), c.path)

				require.NoError(t, err)
				assert.Equal(t, c.want, location, c.path)
			}
		})
	}
}

func TestTheFakeAndGitAgreeOnAnAddAndItsUndo(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			ctx := t.Context()
			before := gittest.StateOf(t, impl.port)

			require.NoError(t, impl.port.AddWorktree(ctx, git.AddWorktreeParams{Path: filepath.Join(impl.root, "topic"), Checkout: git.NewBranch{Name: "topic", From: git.RemoteBranch{Name: "main"}}}))
			added := gittest.StateOf(t, impl.port)
			require.NoError(t, impl.port.RemoveWorktree(ctx, git.RemoveWorktreeParams{Path: filepath.Join(impl.root, "topic"), Force: true}))
			require.NoError(t, impl.port.DeleteBranch(ctx, git.DeleteBranchParams{Name: "topic", Force: true}))

			assert.Equal(t, append(before.Worktrees, git.Worktree{Path: impl.at("topic"), State: git.OnBranch{Branch: "topic"}}), added.Worktrees)
			assert.Contains(t, gittest.Names(added.Branches.Local), "topic")
			assert.Equal(t, before, gittest.StateOf(t, impl.port))
		})
	}
}

func TestTheFakeAndGitStartATrackingBranchWhereTheRemoteOneIs(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			ctx := t.Context()

			added := impl.port.AddWorktree(ctx, git.AddWorktreeParams{Path: filepath.Join(impl.root, "resumed"), Checkout: git.TrackingBranch{Name: "remote-only"}})
			twice := impl.port.AddWorktree(ctx, git.AddWorktreeParams{Path: filepath.Join(impl.root, "again"), Checkout: git.TrackingBranch{Name: "feature"}})

			require.NoError(t, added)
			assert.Equal(t, tip(t, impl.port, git.RemoteBranch{Name: "remote-only"}), tip(t, impl.port, git.LocalBranch{Name: "remote-only"}))
			assert.Contains(t, paths(gittest.StateOf(t, impl.port).Worktrees), impl.at("resumed"))
			require.Error(t, twice)
		})
	}
}

func TestTheFakeAndGitLeaveTheBranchOfAnAddRefusedForItsPath(t *testing.T) {
	for _, impl := range implementations(t) {
		for _, folder := range []string{"taken", "gone"} {
			t.Run(impl.name+": "+folder, func(t *testing.T) {
				before := gittest.StateOf(t, impl.port)

				branch := "into-" + folder
				err := impl.port.AddWorktree(t.Context(), git.AddWorktreeParams{Path: filepath.Join(impl.root, folder), Checkout: git.NewBranch{Name: branch, From: git.LocalBranch{Name: "main"}}})

				require.Error(t, err)
				after := gittest.StateOf(t, impl.port)
				assert.Equal(t, before.Worktrees, after.Worktrees)
				assert.Equal(t, slices.Sorted(slices.Values(append(gittest.Names(before.Branches.Local), branch))), gittest.Names(after.Branches.Local))
			})
		}
	}
}

func TestTheFakeAndGitFetchWhatTheRemoteHas(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			ctx := t.Context()
			pair := git.AheadBehindParams{Of: git.LocalBranch{Name: "main"}, Against: git.RemoteBranch{Name: "main"}}

			require.NoError(t, impl.port.FetchRef(ctx, "main"))
			assert.Equal(t, git.AheadBehind{Behind: 1}, aheadBehind(t, impl.port, pair))
			assert.False(t, refExists(t, impl.port, git.RemoteBranch{Name: "pushed"}))
			require.NoError(t, impl.port.Fetch(ctx))
			assert.True(t, refExists(t, impl.port, git.RemoteBranch{Name: "pushed"}))
			assert.False(t, refExists(t, impl.port, git.LocalBranch{Name: "pushed"}))
		})
	}
}

func TestTheFakeAndGitFastForwardABranchCheckedOutNowhere(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			ctx := t.Context()
			feature := git.LocalBranch{Name: "feature"}
			from := tip(t, impl.port, feature)

			moved, movedErr := impl.port.FastForwardRef(ctx, "feature")
			created, createdErr := impl.port.FastForwardRef(ctx, "pushed")

			require.NoError(t, movedErr)
			assert.Equal(t, git.Move{From: from, To: tip(t, impl.port, git.RemoteBranch{Name: "feature"})}, moved)
			assert.NotEqual(t, moved.From, moved.To)
			assert.Equal(t, git.AheadBehind{}, aheadBehind(t, impl.port, git.AheadBehindParams{Of: feature, Against: git.RemoteBranch{Name: "feature"}}))
			require.NoError(t, createdErr)
			assert.Equal(t, git.Move{To: tip(t, impl.port, git.LocalBranch{Name: "pushed"})}, created)
			assert.Equal(t, tip(t, impl.port, git.RemoteBranch{Name: "pushed"}), created.To)
		})
	}
}

func TestTheFakeAndGitRefuseANonFastForwardButStillFetch(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			local := tip(t, impl.port, git.LocalBranch{Name: "diverged"})
			fetched := tip(t, impl.port, git.RemoteBranch{Name: "diverged"})

			_, err := impl.port.FastForwardRef(t.Context(), "diverged")

			require.Error(t, err)
			assert.Equal(t, local, tip(t, impl.port, git.LocalBranch{Name: "diverged"}))
			assert.NotEqual(t, fetched, tip(t, impl.port, git.RemoteBranch{Name: "diverged"}))
		})
	}
}

func TestTheFakeAndGitMergeAFastForwardInAWorktree(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			ctx := t.Context()
			main := git.LocalBranch{Name: "main"}
			onto := git.MergeFFOnlyParams{Worktree: filepath.Join(impl.root, "main"), Onto: git.RemoteBranch{Name: "main"}}
			from := tip(t, impl.port, main)
			require.NoError(t, impl.port.FetchRef(ctx, "main"))

			moved, movedErr := impl.port.MergeFFOnly(ctx, onto)
			again, againErr := impl.port.MergeFFOnly(ctx, onto)

			require.NoError(t, movedErr)
			assert.Equal(t, git.Move{From: from, To: tip(t, impl.port, git.RemoteBranch{Name: "main"})}, moved)
			assert.Equal(t, moved.To, tip(t, impl.port, main))
			require.NoError(t, againErr)
			assert.Equal(t, git.Move{From: moved.To, To: moved.To}, again)
		})
	}
}

func TestTheFakeAndGitHonourForce(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			ctx := t.Context()

			keptDirty := impl.port.RemoveWorktree(ctx, git.RemoveWorktreeParams{Path: filepath.Join(impl.root, "dirty")})
			forcedDirty := impl.port.RemoveWorktree(ctx, git.RemoveWorktreeParams{Path: filepath.Join(impl.root, "dirty"), Force: true})
			keptUnmerged := impl.port.DeleteBranch(ctx, git.DeleteBranchParams{Name: "unmerged"})
			forcedUnmerged := impl.port.DeleteBranch(ctx, git.DeleteBranchParams{Name: "unmerged", Force: true})
			merged := impl.port.DeleteBranch(ctx, git.DeleteBranchParams{Name: "feature"})
			gone := impl.port.RemoveWorktree(ctx, git.RemoveWorktreeParams{Path: filepath.Join(impl.root, "gone")})

			require.Error(t, keptDirty)
			require.NoError(t, forcedDirty)
			require.Error(t, keptUnmerged)
			require.NoError(t, forcedUnmerged)
			require.NoError(t, merged)
			require.NoError(t, gone)
			assert.NotContains(t, paths(gittest.StateOf(t, impl.port).Worktrees), impl.at("gone"))
		})
	}
}

func TestTheFakeAndGitRefuseTheSameThings(t *testing.T) {
	refusals := []struct {
		name string
		call func(ctx context.Context, impl implementation) error
	}{
		{name: "a branch checked out elsewhere", call: func(ctx context.Context, impl implementation) error {
			return impl.port.AddWorktree(ctx, git.AddWorktreeParams{Path: filepath.Join(impl.root, "new"), Checkout: git.ExistingBranch{Name: "main"}})
		}},
		{name: "a branch a rebase will move", call: func(ctx context.Context, impl implementation) error {
			return impl.port.AddWorktree(ctx, git.AddWorktreeParams{Path: filepath.Join(impl.root, "new"), Checkout: git.ExistingBranch{Name: "rebasing"}})
		}},
		{name: "a branch only the remote has", call: func(ctx context.Context, impl implementation) error {
			return impl.port.AddWorktree(ctx, git.AddWorktreeParams{Path: filepath.Join(impl.root, "new"), Checkout: git.ExistingBranch{Name: "remote-only"}})
		}},
		{name: "an existing branch into a taken folder", call: func(ctx context.Context, impl implementation) error {
			return impl.port.AddWorktree(ctx, git.AddWorktreeParams{Path: filepath.Join(impl.root, "taken"), Checkout: git.ExistingBranch{Name: "feature"}})
		}},
		{name: "a new branch that exists", call: func(ctx context.Context, impl implementation) error {
			return impl.port.AddWorktree(ctx, git.AddWorktreeParams{Path: filepath.Join(impl.root, "new"), Checkout: git.NewBranch{Name: "feature", From: git.LocalBranch{Name: "main"}}})
		}},
		{name: "a start point that does not exist", call: func(ctx context.Context, impl implementation) error {
			return impl.port.AddWorktree(ctx, git.AddWorktreeParams{Path: filepath.Join(impl.root, "new"), Checkout: git.NewBranch{Name: "topic", From: git.RemoteBranch{Name: "nope"}}})
		}},
		{name: "an existing branch that does not exist", call: func(ctx context.Context, impl implementation) error {
			return impl.port.AddWorktree(ctx, git.AddWorktreeParams{Path: filepath.Join(impl.root, "new"), Checkout: git.ExistingBranch{Name: "nope"}})
		}},
		{name: "removing the main worktree", call: func(ctx context.Context, impl implementation) error {
			return impl.port.RemoveWorktree(ctx, git.RemoveWorktreeParams{Path: filepath.Join(impl.root, "main"), Force: true})
		}},
		{name: "removing a locked worktree, even forced", call: func(ctx context.Context, impl implementation) error {
			return impl.port.RemoveWorktree(ctx, git.RemoveWorktreeParams{Path: filepath.Join(impl.root, "locked"), Force: true})
		}},
		{name: "removing a worktree with changes", call: func(ctx context.Context, impl implementation) error {
			return impl.port.RemoveWorktree(ctx, git.RemoveWorktreeParams{Path: filepath.Join(impl.root, "dirty")})
		}},
		{name: "removing what is not a worktree", call: func(ctx context.Context, impl implementation) error {
			return impl.port.RemoveWorktree(ctx, git.RemoveWorktreeParams{Path: filepath.Join(impl.root, "taken"), Force: true})
		}},
		{name: "deleting a branch checked out", call: func(ctx context.Context, impl implementation) error {
			return impl.port.DeleteBranch(ctx, git.DeleteBranchParams{Name: "main", Force: true})
		}},
		{name: "deleting a branch a rebase will move", call: func(ctx context.Context, impl implementation) error {
			return impl.port.DeleteBranch(ctx, git.DeleteBranchParams{Name: "rebasing", Force: true})
		}},
		{name: "deleting an unmerged branch", call: func(ctx context.Context, impl implementation) error {
			return impl.port.DeleteBranch(ctx, git.DeleteBranchParams{Name: "unmerged"})
		}},
		{name: "deleting a branch that does not exist", call: func(ctx context.Context, impl implementation) error {
			return impl.port.DeleteBranch(ctx, git.DeleteBranchParams{Name: "nope", Force: true})
		}},
		{name: "counting against an unknown ref", call: func(ctx context.Context, impl implementation) error {
			_, err := impl.port.AheadBehind(ctx, git.AheadBehindParams{Of: git.LocalBranch{Name: "main"}, Against: git.RemoteBranch{Name: "nope"}})
			return err
		}},
		{name: "fetching a branch the remote lacks", call: func(ctx context.Context, impl implementation) error {
			return impl.port.FetchRef(ctx, "nope")
		}},
		{name: "fast-forwarding a branch checked out", call: func(ctx context.Context, impl implementation) error {
			_, err := impl.port.FastForwardRef(ctx, "main")
			return err
		}},
		{name: "merging onto an unknown ref", call: func(ctx context.Context, impl implementation) error {
			_, err := impl.port.MergeFFOnly(ctx, git.MergeFFOnlyParams{Worktree: filepath.Join(impl.root, "main"), Onto: git.RemoteBranch{Name: "nope"}})
			return err
		}},
		{name: "merging what is not a fast-forward", call: func(ctx context.Context, impl implementation) error {
			_, err := impl.port.MergeFFOnly(ctx, git.MergeFFOnlyParams{Worktree: filepath.Join(impl.root, "linked"), Onto: git.LocalBranch{Name: "unmerged"}})
			return err
		}},
	}
	for _, impl := range implementations(t) {
		for _, refusal := range refusals {
			t.Run(impl.name+": "+refusal.name, func(t *testing.T) {
				before := gittest.StateOf(t, impl.port)

				err := refusal.call(t.Context(), impl)

				require.Error(t, err)
				assert.Equal(t, before, gittest.StateOf(t, impl.port))
			})
		}
	}
}
