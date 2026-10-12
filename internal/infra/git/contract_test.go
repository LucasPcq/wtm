package git_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/infra/git"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

// The double a command is tested on behaves as git does: each case runs on a
// real repository and on gittest.Fake, from the same start (main checked out,
// feature beside it, main on the remote) and must end the same.

type port interface {
	git.Reader
	git.Writer
}

type implementation struct {
	name string
	port port
	dir  string
}

func implementations(t *testing.T) []implementation {
	t.Helper()
	repo, _ := newRepoWithOrigin(t)
	gittest.CreateBranch(t, repo.Dir, "feature")
	fakeDir := tempDir(t)
	fake := gittest.NewFake(gittest.FakeParams{Main: fakeDir, Branch: "main", Local: []string{"feature"}, Remote: []string{"main"}})
	return []implementation{
		{name: "git", port: repo, dir: repo.Dir},
		{name: "fake", port: fake, dir: fakeDir},
	}
}

func TestTheFakeAndGitAgreeOnAnAddAndItsUndo(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			ctx := t.Context()
			before := snapshot(t, impl.port)
			path := filepath.Join(tempDir(t), "topic")

			require.NoError(t, impl.port.AddWorktree(ctx, git.AddWorktreeParams{Path: path, Checkout: git.NewBranch{Name: "topic", From: git.RemoteBranch{Name: "main"}}}))
			added := snapshot(t, impl.port)
			require.NoError(t, impl.port.RemoveWorktree(ctx, git.RemoveWorktreeParams{Path: path, Force: true}))
			require.NoError(t, impl.port.DeleteBranch(ctx, git.DeleteBranchParams{Name: "topic", Force: true}))

			assert.Equal(t, append(before.worktrees, git.Worktree{Path: path, State: git.OnBranch{Branch: "topic"}}), added.worktrees)
			assert.Equal(t, []string{"feature", "main", "topic"}, added.branches.Local)
			assert.Equal(t, before, snapshot(t, impl.port))
		})
	}
}

func TestTheFakeAndGitAgreeOnWhatTheyRead(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			ctx := t.Context()

			main, mainErr := impl.port.MainWorktree(ctx)
			local, localErr := impl.port.RefExists(ctx, git.LocalBranch{Name: "feature"})
			remote, remoteErr := impl.port.RefExists(ctx, git.RemoteBranch{Name: "feature"})
			counts, countsErr := impl.port.AheadBehind(ctx, git.AheadBehindParams{Of: git.LocalBranch{Name: "main"}, Against: git.RemoteBranch{Name: "main"}})

			require.NoError(t, mainErr)
			assert.Equal(t, impl.dir, main.Path)
			require.NoError(t, localErr)
			assert.True(t, local)
			require.NoError(t, remoteErr)
			assert.False(t, remote)
			require.NoError(t, countsErr)
			assert.Equal(t, git.AheadBehind{}, counts)
			assert.Equal(t, git.Branches{Local: []string{"feature", "main"}, Remote: []string{"main"}}, snapshot(t, impl.port).branches)
		})
	}
}

func TestTheFakeAndGitRefuseTheSameThings(t *testing.T) {
	refusals := []struct {
		name string
		call func(ctx context.Context, impl implementation) error
	}{
		{name: "a branch checked out elsewhere", call: func(ctx context.Context, impl implementation) error {
			return impl.port.AddWorktree(ctx, git.AddWorktreeParams{Path: filepath.Join(t.TempDir(), "main"), Checkout: git.ExistingBranch{Name: "main"}})
		}},
		{name: "a new branch that exists", call: func(ctx context.Context, impl implementation) error {
			return impl.port.AddWorktree(ctx, git.AddWorktreeParams{Path: filepath.Join(t.TempDir(), "feature"), Checkout: git.NewBranch{Name: "feature", From: git.LocalBranch{Name: "main"}}})
		}},
		{name: "a start point that does not exist", call: func(ctx context.Context, impl implementation) error {
			return impl.port.AddWorktree(ctx, git.AddWorktreeParams{Path: filepath.Join(t.TempDir(), "topic"), Checkout: git.NewBranch{Name: "topic", From: git.RemoteBranch{Name: "feature"}}})
		}},
		{name: "an existing branch that does not exist", call: func(ctx context.Context, impl implementation) error {
			return impl.port.AddWorktree(ctx, git.AddWorktreeParams{Path: filepath.Join(t.TempDir(), "nope"), Checkout: git.ExistingBranch{Name: "nope"}})
		}},
		{name: "removing the main worktree", call: func(ctx context.Context, impl implementation) error {
			return impl.port.RemoveWorktree(ctx, git.RemoveWorktreeParams{Path: impl.dir, Force: true})
		}},
		{name: "removing what is not a worktree", call: func(ctx context.Context, impl implementation) error {
			return impl.port.RemoveWorktree(ctx, git.RemoveWorktreeParams{Path: t.TempDir(), Force: true})
		}},
		{name: "deleting a branch checked out", call: func(ctx context.Context, impl implementation) error {
			return impl.port.DeleteBranch(ctx, git.DeleteBranchParams{Name: "main", Force: true})
		}},
		{name: "deleting a branch that does not exist", call: func(ctx context.Context, impl implementation) error {
			return impl.port.DeleteBranch(ctx, git.DeleteBranchParams{Name: "nope", Force: true})
		}},
		{name: "counting against an unknown ref", call: func(ctx context.Context, impl implementation) error {
			_, err := impl.port.AheadBehind(ctx, git.AheadBehindParams{Of: git.LocalBranch{Name: "main"}, Against: git.RemoteBranch{Name: "feature"}})
			return err
		}},
		{name: "fetching a branch the remote lacks", call: func(ctx context.Context, impl implementation) error {
			return impl.port.FetchRef(ctx, "feature")
		}},
		{name: "fast-forwarding a branch checked out", call: func(ctx context.Context, impl implementation) error {
			return impl.port.FastForwardRef(ctx, "main")
		}},
		{name: "merging onto an unknown ref", call: func(ctx context.Context, impl implementation) error {
			return impl.port.MergeFFOnly(ctx, git.MergeFFOnlyParams{Worktree: impl.dir, Onto: git.RemoteBranch{Name: "feature"}})
		}},
	}
	for _, impl := range implementations(t) {
		for _, refusal := range refusals {
			t.Run(impl.name+": "+refusal.name, func(t *testing.T) {
				before := snapshot(t, impl.port)

				err := refusal.call(t.Context(), impl)

				require.Error(t, err)
				assert.Equal(t, before, snapshot(t, impl.port))
			})
		}
	}
}

type state struct {
	worktrees []git.Worktree
	branches  git.Branches
}

// snapshot leaves out each Head: the fake has no commits.
func snapshot(t *testing.T, port port) state {
	t.Helper()
	worktrees, err := port.Worktrees(t.Context())
	require.NoError(t, err)
	branches, err := port.Branches(t.Context())
	require.NoError(t, err)
	for i := range worktrees {
		worktrees[i].Head = ""
	}
	return state{worktrees: worktrees, branches: branches}
}
