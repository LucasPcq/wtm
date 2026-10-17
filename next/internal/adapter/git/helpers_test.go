package git_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/git"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"

	"github.com/LucasPcq/wtm/next/internal/wtm/worktree"
	"github.com/stretchr/testify/require"
)

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

func advanceOnOrigin(t *testing.T, origin, name string) string {
	t.Helper()
	clone := filepath.Join(t.TempDir(), "clone")
	gittest.Git(t, t.TempDir(), "clone", origin, clone)
	gittest.Git(t, clone, "config", "user.email", "test@test.com")
	gittest.Git(t, clone, "config", "user.name", "Test")
	gittest.Git(t, clone, "checkout", "-B", name, "origin/"+name)
	gittest.Git(t, clone, "commit", "--allow-empty", "-m", "upstream work")
	gittest.Git(t, clone, "push", "origin", name)
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

func rebase(t *testing.T, dir, backend string) {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "rebase", backend, "main").CombinedOutput()
	require.Error(t, err, "the rebase was meant to stop on a conflict: %s", out)
}

func upstreamOf(t *testing.T, dir, name string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "config", "branch."+name+".merge").Output()
	if err != nil {
		return ""
	}
	return string(out)
}

func paths(worktrees []worktree.Worktree) []string {
	var all []string
	for _, w := range worktrees {
		all = append(all, w.Path)
	}
	return all
}

func tipsOf(t *testing.T, reader port) map[worktree.Ref]string {
	t.Helper()
	branches, err := reader.Branches(t.Context())
	require.NoError(t, err)
	tips := map[worktree.Ref]string{}
	for _, b := range branches.Local {
		tips[worktree.LocalBranch{Name: b.Name}] = b.Tip
	}
	for _, b := range branches.Remote {
		tips[worktree.RemoteBranch{Name: b.Name}] = b.Tip
	}
	return tips
}

func has(t *testing.T, reader port, ref worktree.Ref) bool {
	t.Helper()
	_, ok := tipsOf(t, reader)[ref]
	return ok
}

func tip(t *testing.T, reader port, ref worktree.Ref) string {
	t.Helper()
	commit, ok := tipsOf(t, reader)[ref]
	require.True(t, ok, "no ref %v", ref)
	return commit
}

func aheadBehind(t *testing.T, reader port, name string) worktree.AheadBehind {
	t.Helper()
	counts, err := reader.AheadBehind(t.Context(), name)
	require.NoError(t, err)
	return counts
}

func named(names ...string) []worktree.Branch {
	branches := make([]worktree.Branch, 0, len(names))
	for _, name := range names {
		branches = append(branches, worktree.Branch{Name: name})
	}
	return branches
}

func leaveAFile(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		return
	}
	require.NoError(t, os.WriteFile(filepath.Join(path, ".env"), []byte("PORT=3000\n"), 0o644))
}

func fakeSSH(t *testing.T) string {
	t.Helper()
	bin := t.TempDir()
	record := filepath.Join(t.TempDir(), "ssh-args")
	script := "#!/bin/sh\necho \"$@\" >> " + record + "\nexit 255\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, "ssh"), []byte(script), 0o755))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GIT_SSH_COMMAND", "")
	require.NoError(t, os.Unsetenv("GIT_SSH_COMMAND"))
	return record
}

func contentOf(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

func lastLine(content string) string {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	return lines[len(lines)-1]
}
