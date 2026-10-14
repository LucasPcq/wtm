package git_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/infra/git"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

const remote = "origin"

func newRepo(t *testing.T) git.Repo {
	t.Helper()
	return git.Repo{Dir: gittest.Resolved(t, gittest.InitRepo(t)), Remote: remote}
}

func newRepoWithOrigin(t *testing.T) (git.Repo, string) {
	t.Helper()
	repo := newRepo(t)
	return repo, gittest.AddOrigin(t, repo.Dir)
}

func commitFile(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
	gittest.Git(t, dir, "add", name)
	gittest.Git(t, dir, "commit", "-m", name+": "+content)
}

func head(t *testing.T, dir, ref string) string {
	t.Helper()
	return gittest.Output(t, dir, "rev-parse", ref)
}

// advanceOnOrigin commits to branch on the remote from a clone of its own, so
// the repository under test only learns of it by fetching.
func advanceOnOrigin(t *testing.T, origin, branch string) string {
	t.Helper()
	clone := filepath.Join(t.TempDir(), "clone")
	gittest.Git(t, t.TempDir(), "clone", origin, clone)
	gittest.Git(t, clone, "config", "user.email", "test@test.com")
	gittest.Git(t, clone, "config", "user.name", "Test")
	gittest.Git(t, clone, "checkout", "-B", branch, "origin/"+branch)
	gittest.Git(t, clone, "commit", "--allow-empty", "-m", "upstream work")
	gittest.Git(t, clone, "push", "origin", branch)
	return head(t, clone, "HEAD")
}

func checkOutFeature(t *testing.T, dir string, linked bool) string {
	t.Helper()
	if !linked {
		gittest.Git(t, dir, "checkout", "feature")
		return dir
	}
	where := filepath.Join(gittest.TempDir(t), "feature")
	gittest.Git(t, dir, "worktree", "add", where, "feature")
	return where
}

// rebase onto main stops on a conflict, leaving the rebase in progress.
func rebase(t *testing.T, dir, backend string) {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "rebase", backend, "main").CombinedOutput()
	require.Error(t, err, "the rebase was meant to stop on a conflict: %s", out)
}

func upstreamOf(t *testing.T, dir, branch string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "config", "branch."+branch+".merge").Output()
	if err != nil {
		return ""
	}
	return string(out)
}

func paths(worktrees []git.Worktree) []string {
	var all []string
	for _, worktree := range worktrees {
		all = append(all, worktree.Path)
	}
	return all
}

func refExists(t *testing.T, reader git.Reader, ref git.Ref) bool {
	t.Helper()
	exists, err := reader.RefExists(t.Context(), ref)
	require.NoError(t, err)
	return exists
}

func tip(t *testing.T, reader git.Reader, ref git.Ref) string {
	t.Helper()
	commit, err := reader.Tip(t.Context(), ref)
	require.NoError(t, err)
	return commit
}

func aheadBehind(t *testing.T, reader git.Reader, params git.AheadBehindParams) git.AheadBehind {
	t.Helper()
	counts, err := reader.AheadBehind(t.Context(), params)
	require.NoError(t, err)
	return counts
}
