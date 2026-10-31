package git_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/git"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/stretchr/testify/require"
)

func newRepo(t *testing.T) git.Repo {
	t.Helper()
	return git.Repo{Dir: gittest.Resolved(t, gittest.InitRepo(t))}
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

// advanceOnOrigin pushes a new commit to origin's branch name from a clone
// and returns that commit.
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

// checkOutFeature checks feature out in the main worktree, or in a new linked
// one, and returns where.
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

// rebase starts a rebase onto main with backend, which must stop on a conflict.
func rebase(t *testing.T, dir, backend string) {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "rebase", backend, "main").CombinedOutput()
	require.Error(t, err, "the rebase was meant to stop on a conflict: %s", out)
}

// stopped runs git with args in dir, which must stop on a conflict.
func stopped(t *testing.T, dir string, args ...string) {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	require.Error(t, err, "git %v was meant to stop on a conflict: %s", args, out)
}

// merging, cherryPicking, reverting and applying stop git in the middle of a merge, a cherry-pick, a revert or a git am, on a conflict between feature checked out at where and main.
func merging(t *testing.T, _, where string) { stopped(t, where, "merge", "main") }

func cherryPicking(t *testing.T, _, where string) { stopped(t, where, "cherry-pick", "main") }

func reverting(t *testing.T, _, where string) {
	commitFile(t, where, "file", "feature again")
	stopped(t, where, "revert", "--no-edit", "HEAD~1")
}

func applying(t *testing.T, repo, where string) {
	patch := filepath.Join(gittest.TempDir(t), "main.patch")
	require.NoError(t, os.WriteFile(patch, []byte(gittest.Output(t, repo, "format-patch", "-1", "main", "--stdout")+"\n"), 0o644))
	stopped(t, where, "am", patch)
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

// leaveAFile writes a .env inside the directory at path, if it exists.
func leaveAFile(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		return
	}
	require.NoError(t, os.WriteFile(filepath.Join(path, ".env"), []byte("PORT=3000\n"), 0o644))
}

// fakeSSH puts an ssh on PATH that records its arguments and fails, with
// GIT_SSH_COMMAND unset, and returns the file it records to.
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

// pair is a repository with two worktrees at the same commit: source, the main
// one, and target, a linked one on branch split.
type pair struct {
	repo   git.Repo
	source string
	target string
}

const numbered = "1\n2\n3\n4\n5\n6\n7\n8\n9\n"

// sourceAndTarget builds a pair whose base commit holds a.txt, c.txt,
// gone.txt, README.md and a .gitignore ignoring ignored.log.
func sourceAndTarget(t *testing.T) pair {
	t.Helper()
	repo := newRepo(t)
	for name, content := range map[string]string{"a.txt": numbered, "c.txt": numbered, "gone.txt": "gone\n", "README.md": "readme\n", ".gitignore": "ignored.log\n"} {
		write(t, filepath.Join(repo.Dir, name), content)
	}
	gittest.Git(t, repo.Dir, "add", "-A")
	gittest.Git(t, repo.Dir, "commit", "-q", "-m", "base")
	target := filepath.Join(gittest.TempDir(t), "split")
	gittest.Git(t, repo.Dir, "worktree", "add", "-q", "-b", "split", target)
	return pair{repo: repo, source: repo.Dir, target: target}
}

func (p pair) move(changes ...worktree.Change) worktree.Move {
	return worktree.Move{Source: p.source, Target: p.target, Changes: changes, SourceLabel: "main", TargetLabel: "split"}
}

func (p pair) check(changes ...worktree.Change) worktree.ConflictCheck {
	return worktree.ConflictCheck{Source: p.source, Target: p.target, Changes: changes}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func replaceLine(content string, line int, with string) string {
	lines := strings.Split(content, "\n")
	lines[line-1] = with
	return strings.Join(lines, "\n")
}

func porcelain(t *testing.T, dir string) []string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "status", "--porcelain", "--untracked-files=all").Output()
	require.NoError(t, err)
	return strings.FieldsFunc(string(out), func(r rune) bool { return r == '\n' })
}

// childOfMain is a repository with an origin, its main holding shared.txt, and a worktree on child, a branch from main with a commit of its own.
func childOfMain(t *testing.T) (git.Repo, string) {
	t.Helper()
	repo, _ := newRepoWithOrigin(t)
	commitFile(t, repo.Dir, "shared.txt", "base")
	child := filepath.Join(gittest.TempDir(t), "child")
	gittest.Git(t, repo.Dir, "worktree", "add", "-q", "-b", "child", child, "main")
	commitFile(t, child, "child.txt", "its own")
	return repo, child
}

// conflictingChild is childOfMain where child and main then each change shared.txt their own way.
func conflictingChild(t *testing.T) (git.Repo, string) {
	t.Helper()
	repo, child := childOfMain(t)
	commitFile(t, child, "shared.txt", "child's line")
	commitFile(t, repo.Dir, "shared.txt", "main's line")
	return repo, child
}

// resolveByHand rebases child onto main, stops on its conflict, and resolves it as a user does: a new line, added, then continued.
func resolveByHand(t *testing.T, repo git.Repo, child string) {
	t.Helper()
	_, err := repo.Rebase(t.Context(), worktree.Rebase{Path: child, Onto: worktree.LocalBranch{Name: "main"}, Upstream: forkOf(t, repo, "child", "main"), Keep: true})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(child, "shared.txt"), []byte("resolved\n"), 0o644))
	gittest.Git(t, child, "add", "shared.txt")
	gittest.Git(t, child, "-c", "core.editor=true", "rebase", "--continue")
}

func forkOf(t *testing.T, repo git.Repo, branch, base string) string {
	t.Helper()
	fork, err := repo.ForkPoint(t.Context(), worktree.Span{Branch: branch, Base: base})
	require.NoError(t, err)
	return fork.Commit
}
