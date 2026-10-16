package git_test

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/git"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/wtm/branch"
	"github.com/LucasPcq/wtm/next/internal/wtm/worktree"
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
	from := head(t, repo.Dir, "feature")

	move, err := repo.FastForwardRef(t.Context(), "feature")

	require.NoError(t, err)
	assert.Equal(t, branch.Move{From: from, To: pushed}, move)
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

	_, diverged := repo.FastForwardRef(t.Context(), "diverged")
	_, checkedOut := repo.FastForwardRef(t.Context(), "main")

	var gitErr *git.Error
	require.ErrorAs(t, diverged, &gitErr)
	require.ErrorAs(t, checkedOut, &gitErr)
}

func TestMergeFFOnlyMovesTheBranchOfAWorktree(t *testing.T) {
	repo, origin := newRepoWithOrigin(t)
	pushed := advanceOnOrigin(t, origin, "main")
	require.NoError(t, repo.FetchRef(t.Context(), "main"))
	from := head(t, repo.Dir, "main")

	move, err := repo.MergeFFOnly(t.Context(), worktree.Merge{Path: repo.Dir, Onto: branch.RemoteBranch{Name: "main"}})

	require.NoError(t, err)
	assert.Equal(t, branch.Move{From: from, To: pushed}, move)
	assert.Equal(t, pushed, head(t, repo.Dir, "main"))
}

func TestMergeFFOnlyRefusesWhatIsNotAFastForward(t *testing.T) {
	repo, origin := newRepoWithOrigin(t)
	commitFile(t, repo.Dir, "local", "1")
	advanceOnOrigin(t, origin, "main")
	require.NoError(t, repo.FetchRef(t.Context(), "main"))

	_, diverged := repo.MergeFFOnly(t.Context(), worktree.Merge{Path: repo.Dir, Onto: branch.RemoteBranch{Name: "main"}})
	_, noOnto := repo.MergeFFOnly(t.Context(), worktree.Merge{Path: repo.Dir})

	var gitErr *git.Error
	require.ErrorAs(t, diverged, &gitErr)
	require.Error(t, noOnto)
}

func TestTheFakeAndGitFetchWhatTheRemoteHas(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			pair := branch.AheadBehindParams{Of: branch.LocalBranch{Name: "main"}, Against: branch.RemoteBranch{Name: "main"}}

			require.NoError(t, impl.port.FetchRef(t.Context(), "main"))

			assert.Equal(t, branch.AheadBehind{Behind: 1}, aheadBehind(t, impl.port, pair))
			assert.False(t, has(t, impl.port, branch.RemoteBranch{Name: "pushed"}))
		})
	}
}

func TestTheFakeAndGitFastForwardABranchCheckedOutNowhere(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			feature := branch.LocalBranch{Name: "feature"}
			from := tip(t, impl.port, feature)

			moved, movedErr := impl.port.FastForwardRef(t.Context(), "feature")
			created, createdErr := impl.port.FastForwardRef(t.Context(), "pushed")

			require.NoError(t, movedErr)
			assert.Equal(t, branch.Move{From: from, To: tip(t, impl.port, branch.RemoteBranch{Name: "feature"})}, moved)
			assert.NotEqual(t, moved.From, moved.To)
			assert.Equal(t, branch.AheadBehind{}, aheadBehind(t, impl.port, branch.AheadBehindParams{Of: feature, Against: branch.RemoteBranch{Name: "feature"}}))
			require.NoError(t, createdErr)
			assert.Equal(t, branch.Move{To: tip(t, impl.port, branch.LocalBranch{Name: "pushed"})}, created)
			assert.Equal(t, tip(t, impl.port, branch.RemoteBranch{Name: "pushed"}), created.To)
		})
	}
}

func TestTheFakeAndGitRefuseANonFastForwardButStillFetch(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			local := tip(t, impl.port, branch.LocalBranch{Name: "diverged"})
			fetched := tip(t, impl.port, branch.RemoteBranch{Name: "diverged"})

			_, err := impl.port.FastForwardRef(t.Context(), "diverged")

			require.Error(t, err)
			assert.Equal(t, local, tip(t, impl.port, branch.LocalBranch{Name: "diverged"}))
			assert.NotEqual(t, fetched, tip(t, impl.port, branch.RemoteBranch{Name: "diverged"}))
		})
	}
}

func TestTheFakeAndGitMergeAFastForwardInAWorktree(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			main := branch.LocalBranch{Name: "main"}
			onto := worktree.Merge{Path: filepath.Join(impl.root, "main"), Onto: branch.RemoteBranch{Name: "main"}}
			from := tip(t, impl.port, main)
			require.NoError(t, impl.port.FetchRef(t.Context(), "main"))

			moved, movedErr := impl.port.MergeFFOnly(t.Context(), onto)
			again, againErr := impl.port.MergeFFOnly(t.Context(), onto)

			require.NoError(t, movedErr)
			assert.Equal(t, branch.Move{From: from, To: tip(t, impl.port, branch.RemoteBranch{Name: "main"})}, moved)
			assert.Equal(t, moved.To, tip(t, impl.port, main))
			require.NoError(t, againErr)
			assert.Equal(t, branch.Move{From: moved.To, To: moved.To}, again)
		})
	}
}

func TestTheFakeAndGitRefuseTheSameRemoteChanges(t *testing.T) {
	refusals := []struct {
		name string
		call func(t *testing.T, impl implementation) error
	}{
		{name: "fetching a branch the remote lacks", call: func(t *testing.T, impl implementation) error {
			return impl.port.FetchRef(t.Context(), "nope")
		}},
		{name: "fast-forwarding a branch checked out", call: func(t *testing.T, impl implementation) error {
			_, err := impl.port.FastForwardRef(t.Context(), "main")
			return err
		}},
		{name: "merging onto an unknown ref", call: func(t *testing.T, impl implementation) error {
			_, err := impl.port.MergeFFOnly(t.Context(), worktree.Merge{Path: filepath.Join(impl.root, "main"), Onto: branch.RemoteBranch{Name: "nope"}})
			return err
		}},
		{name: "merging what is not a fast-forward", call: func(t *testing.T, impl implementation) error {
			_, err := impl.port.MergeFFOnly(t.Context(), worktree.Merge{Path: filepath.Join(impl.root, "linked"), Onto: branch.LocalBranch{Name: "unmerged"}})
			return err
		}},
	}
	for _, impl := range implementations(t) {
		for _, refusal := range refusals {
			t.Run(impl.name+": "+refusal.name, func(t *testing.T) {
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
