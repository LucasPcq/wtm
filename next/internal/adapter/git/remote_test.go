package git_test

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/git"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchRefMovesTheRemoteBranchAndNotTheLocalOne(t *testing.T) {
	repo, origin := newRepoWithOrigin(t)
	local := head(t, repo.Dir, "main")
	pushed := advanceOnOrigin(t, origin, "main")

	require.NoError(t, repo.FetchRef(t.Context(), "main"))

	assert.Equal(t, pushed, head(t, repo.Dir, "origin/main"))
	assert.Equal(t, local, head(t, repo.Dir, "main"))
}

func TestFastForwardRefMovesABranchCheckedOutNowhere(t *testing.T) {
	repo, origin := newRepoWithOrigin(t)
	gittest.CreateBranch(t, repo.Dir, "feature")
	gittest.PushBranch(t, repo.Dir, "feature")
	pushed := advanceOnOrigin(t, origin, "feature")

	require.NoError(t, repo.FastForwardRef(t.Context(), "feature"))

	assert.Equal(t, pushed, head(t, repo.Dir, "feature"))
}

func TestFastForwardRefRefusesWhatIsNotAFastForward(t *testing.T) {
	repo, origin := newRepoWithOrigin(t)
	gittest.Git(t, repo.Dir, "checkout", "-b", "diverged")
	gittest.PushBranch(t, repo.Dir, "diverged")
	commitFile(t, repo.Dir, "local", "1")
	gittest.Git(t, repo.Dir, "checkout", "main")
	advanceOnOrigin(t, origin, "diverged")
	advanceOnOrigin(t, origin, "main")

	diverged := repo.FastForwardRef(t.Context(), "diverged")
	checkedOut := repo.FastForwardRef(t.Context(), "main")

	var gitErr *git.Error
	require.ErrorAs(t, diverged, &gitErr)
	require.ErrorAs(t, checkedOut, &gitErr)
}

func TestMergeFFOnlyMovesTheBranchOfAWorktree(t *testing.T) {
	repo, origin := newRepoWithOrigin(t)
	pushed := advanceOnOrigin(t, origin, "main")
	require.NoError(t, repo.FetchRef(t.Context(), "main"))

	require.NoError(t, repo.MergeFFOnly(t.Context(), worktree.Merge{Path: repo.Dir, Onto: worktree.RemoteBranch{Name: "main"}}))

	assert.Equal(t, pushed, head(t, repo.Dir, "main"))
}

func TestMergeFFOnlyRefusesWhatIsNotAFastForward(t *testing.T) {
	repo, origin := newRepoWithOrigin(t)
	commitFile(t, repo.Dir, "local", "1")
	advanceOnOrigin(t, origin, "main")
	require.NoError(t, repo.FetchRef(t.Context(), "main"))

	diverged := repo.MergeFFOnly(t.Context(), worktree.Merge{Path: repo.Dir, Onto: worktree.RemoteBranch{Name: "main"}})
	noOnto := repo.MergeFFOnly(t.Context(), worktree.Merge{Path: repo.Dir})

	var gitErr *git.Error
	require.ErrorAs(t, diverged, &gitErr)
	require.Error(t, noOnto)
}

func TestTheFakeAndGitFetchWhatTheRemoteHas(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			require.NoError(t, impl.port.FetchRef(t.Context(), "main"))

			assert.Equal(t, worktree.AheadBehind{Behind: 1}, aheadBehind(t, impl.port, "main"))
			assert.False(t, has(t, impl.port, worktree.RemoteBranch{Name: "pushed"}))
		})
	}
}

func TestTheFakeAndGitFastForwardABranchCheckedOutNowhere(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			feature := worktree.LocalBranch{Name: "feature"}
			from := tip(t, impl.port, feature)

			movedErr := impl.port.FastForwardRef(t.Context(), "feature")
			createdErr := impl.port.FastForwardRef(t.Context(), "pushed")

			require.NoError(t, movedErr)
			assert.NotEqual(t, from, tip(t, impl.port, feature))
			assert.Equal(t, tip(t, impl.port, worktree.RemoteBranch{Name: "feature"}), tip(t, impl.port, feature))
			assert.Equal(t, worktree.AheadBehind{}, aheadBehind(t, impl.port, "feature"))
			require.NoError(t, createdErr)
			assert.Equal(t, tip(t, impl.port, worktree.RemoteBranch{Name: "pushed"}), tip(t, impl.port, worktree.LocalBranch{Name: "pushed"}))
		})
	}
}

func TestTheFakeAndGitRefuseANonFastForwardButStillFetch(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			local := tip(t, impl.port, worktree.LocalBranch{Name: "diverged"})
			fetched := tip(t, impl.port, worktree.RemoteBranch{Name: "diverged"})

			err := impl.port.FastForwardRef(t.Context(), "diverged")

			require.Error(t, err)
			assert.Equal(t, local, tip(t, impl.port, worktree.LocalBranch{Name: "diverged"}))
			assert.NotEqual(t, fetched, tip(t, impl.port, worktree.RemoteBranch{Name: "diverged"}))
		})
	}
}

func TestTheFakeAndGitMergeAFastForwardInAWorktree(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			main := worktree.LocalBranch{Name: "main"}
			onto := worktree.Merge{Path: filepath.Join(impl.root, "main"), Onto: worktree.RemoteBranch{Name: "main"}}
			from := tip(t, impl.port, main)
			require.NoError(t, impl.port.FetchRef(t.Context(), "main"))

			movedErr := impl.port.MergeFFOnly(t.Context(), onto)
			moved := tip(t, impl.port, main)
			againErr := impl.port.MergeFFOnly(t.Context(), onto)

			require.NoError(t, movedErr)
			assert.NotEqual(t, from, moved)
			assert.Equal(t, tip(t, impl.port, worktree.RemoteBranch{Name: "main"}), moved)
			require.NoError(t, againErr)
			assert.Equal(t, moved, tip(t, impl.port, main))
		})
	}
}

func TestTheFakeAndGitRefuseTheSameRemoteChanges(t *testing.T) {
	refusals := []struct {
		name string
		call func(t *testing.T, impl implementation) error
	}{
		{name: "fetch_missing_remote_branch", call: func(t *testing.T, impl implementation) error {
			return impl.port.FetchRef(t.Context(), "nope")
		}},
		{name: "fast_forward_checked_out", call: func(t *testing.T, impl implementation) error {
			err := impl.port.FastForwardRef(t.Context(), "main")
			return err
		}},
		{name: "merge_unknown_ref", call: func(t *testing.T, impl implementation) error {
			err := impl.port.MergeFFOnly(t.Context(), worktree.Merge{Path: filepath.Join(impl.root, "main"), Onto: worktree.RemoteBranch{Name: "nope"}})
			return err
		}},
		{name: "merge_not_fast_forward", call: func(t *testing.T, impl implementation) error {
			err := impl.port.MergeFFOnly(t.Context(), worktree.Merge{Path: filepath.Join(impl.root, "linked"), Onto: worktree.LocalBranch{Name: "unmerged"}})
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

func TestAFetchThatWouldPromptForCredentialsFailsAtOnce(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("WWW-Authenticate", `Basic realm="git"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)
	repo := newRepo(t)
	gittest.Git(t, repo.Dir, "remote", "add", "origin", server.URL+"/repo.git")

	begin := time.Now()
	err := repo.FetchRef(t.Context(), "main")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "terminal prompts disabled")
	assert.Less(t, time.Since(begin), 5*time.Second)
}

func TestAnSSHFetchNeverAsksAndKeepsTheUsersOwnSSHCommand(t *testing.T) {
	record := fakeSSH(t)
	repo := newRepo(t)
	gittest.Git(t, repo.Dir, "remote", "add", "origin", "ssh://git@wtm-next.invalid/repo.git")

	_ = repo.FetchRef(t.Context(), "main")
	assert.Contains(t, contentOf(t, record), "BatchMode=yes")

	t.Setenv("GIT_SSH_COMMAND", "ssh -o User=mine")
	_ = repo.FetchRef(t.Context(), "main")
	assert.Contains(t, contentOf(t, record), "User=mine")
	assert.NotContains(t, lastLine(contentOf(t, record)), "BatchMode")
}

func TestFetchRefFetchesTheBranchWhenATagHasItsName(t *testing.T) {
	repo, origin := newRepoWithOrigin(t)
	gittest.CreateBranch(t, repo.Dir, "rel")
	gittest.PushBranch(t, repo.Dir, "rel")
	pushed := advanceOnOrigin(t, origin, "rel")
	gittest.Git(t, repo.Dir, "tag", "rel")
	gittest.Git(t, repo.Dir, "push", "-q", "origin", "refs/tags/rel")

	require.NoError(t, repo.FetchRef(t.Context(), "rel"))

	assert.Equal(t, pushed, head(t, repo.Dir, "refs/remotes/origin/rel"))
}

func TestABranchNamedLikeATagIsFetchedAsABranch(t *testing.T) {
	repo, origin := newRepoWithOrigin(t)
	gittest.CreateBranch(t, repo.Dir, "tags/v1")
	gittest.PushBranch(t, repo.Dir, "tags/v1")
	pushed := advanceOnOrigin(t, origin, "tags/v1")

	require.NoError(t, repo.FetchRef(t.Context(), "tags/v1"))
	require.NoError(t, repo.FastForwardRef(t.Context(), "tags/v1"))

	assert.Equal(t, pushed, head(t, repo.Dir, "refs/remotes/origin/tags/v1"))
	assert.Equal(t, pushed, head(t, repo.Dir, "refs/heads/tags/v1"))
	assert.Empty(t, gittest.Output(t, repo.Dir, "tag", "--list"))
}
