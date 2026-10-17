// Package gittest gives tests a git repository, real or in memory.
package gittest

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/git"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

const (
	Root   = "root"
	origin = "origin"
)

func InitRepo(t testing.TB) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-b", "main"},
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "Test"},
		{"commit", "--allow-empty", "-m", "initial"},
	} {
		Git(t, dir, args...)
	}
	return dir
}

func Git(t testing.TB, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %s: %v", strings.Join(args, " "), out, err)
	}
}

func Output(t testing.TB, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out))
}

func CreateBranch(t testing.TB, dir, name string) {
	t.Helper()
	Git(t, dir, "branch", name)
}

func AddOrigin(t testing.TB, dir string) string {
	t.Helper()
	remote := t.TempDir()
	Git(t, remote, "init", "--bare", "-b", "main")
	Git(t, dir, "remote", "add", origin, remote)
	Git(t, dir, "push", "-u", origin, "main")
	return remote
}

func PushBranch(t testing.TB, dir, name string) {
	t.Helper()
	Git(t, dir, "push", "-u", origin, name)
}

func TempDir(t testing.TB) string {
	t.Helper()
	return Resolved(t, t.TempDir())
}

func Resolved(t testing.TB, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func Symlinked(t testing.TB) string {
	t.Helper()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(t.TempDir(), link); err != nil {
		t.Fatal(err)
	}
	return link
}

func Names(branches []worktree.Branch) []string {
	names := make([]string, 0, len(branches))
	for _, b := range branches {
		names = append(names, b.Name)
	}
	return names
}

type State struct {
	Worktrees []worktree.Worktree
	Branches  worktree.Branches
}

type reader interface {
	Worktrees(ctx context.Context) ([]worktree.Worktree, error)
	Branches(ctx context.Context) (worktree.Branches, error)
}

func StateOf(t testing.TB, repo reader) State {
	t.Helper()
	worktrees, err := repo.Worktrees(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	branches, err := repo.Branches(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	return State{Worktrees: worktrees, Branches: worktree.Branches{Local: untipped(branches.Local), Remote: untipped(branches.Remote)}}
}

func untipped(branches []worktree.Branch) []worktree.Branch {
	for i := range branches {
		branches[i].Tip = ""
	}
	return branches
}

func Build(t testing.TB, params FakeParams) git.Repo {
	t.Helper()
	main := params.Main
	if err := os.MkdirAll(main, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q", "-b", params.Branch},
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "Test"},
		{"commit", "-q", "--allow-empty", "-m", Root},
	} {
		Git(t, main, args...)
	}
	commits := builder{t: t, dir: main, made: map[string]string{Root: Output(t, main, "rev-parse", "HEAD")}}
	for _, name := range localBranches(params) {
		Git(t, main, "update-ref", "refs/heads/"+name, commits.tip(params.Commits[worktree.LocalBranch{Name: name}]))
	}
	Git(t, main, "reset", "-q", "--hard")
	buildRemote(t, buildRemoteParams{Main: main, Commits: commits, Seed: params})
	for _, w := range params.Worktrees {
		buildWorktree(t, buildWorktreeParams{Main: main, Root: commits.made[Root], Worktree: w, Seed: params})
	}
	for _, dirty := range params.Dirty {
		writeFile(t, filepath.Join(dirty, "untracked"))
	}
	for _, folder := range params.Folders {
		writeFile(t, filepath.Join(folder, "work"))
	}
	for _, folder := range params.Empty {
		if err := os.MkdirAll(folder, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return git.Repo{Dir: main, Remote: origin}
}

type builder struct {
	t    testing.TB
	dir  string
	made map[string]string
}

func (b builder) tip(commits []string) string {
	b.t.Helper()
	parent := b.made[Root]
	for _, name := range commits {
		made, ok := b.made[name]
		if !ok {
			made = Output(b.t, b.dir, "commit-tree", parent+"^{tree}", "-p", parent, "-m", name)
			b.made[name] = made
		}
		parent = made
	}
	return parent
}

func localBranches(params FakeParams) []string {
	names := append([]string{params.Branch}, params.Local...)
	for _, w := range params.Worktrees {
		if name := w.Branch(); name != "" {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return slices.Compact(names)
}

type buildRemoteParams struct {
	Main    string
	Commits builder
	Seed    FakeParams
}

func buildRemote(t testing.TB, params buildRemoteParams) {
	t.Helper()
	bare := t.TempDir()
	Git(t, bare, "init", "-q", "--bare", "-b", params.Seed.Branch)
	Git(t, params.Main, "remote", "add", origin, bare)
	for _, name := range params.Seed.Remote {
		Git(t, params.Main, "push", "-q", origin, params.Commits.tip(params.Seed.Commits[worktree.RemoteBranch{Name: name}])+":refs/heads/"+name)
	}
	for name, commits := range params.Seed.Origin {
		Git(t, params.Main, "push", "-q", "-f", origin, params.Commits.tip(commits)+":refs/heads/"+name)
	}
	for name := range params.Seed.Origin {
		Git(t, params.Main, "update-ref", "-d", remoteRef(name))
	}
	for _, name := range params.Seed.Remote {
		Git(t, params.Main, "update-ref", remoteRef(name), params.Commits.tip(params.Seed.Commits[worktree.RemoteBranch{Name: name}]))
	}
}

type buildWorktreeParams struct {
	Main     string
	Root     string
	Worktree worktree.Worktree
	Seed     FakeParams
}

func buildWorktree(t testing.TB, params buildWorktreeParams) {
	t.Helper()
	path := params.Worktree.Path
	switch state := params.Worktree.State.(type) {
	case worktree.OnBranch:
		Git(t, params.Main, "worktree", "add", "-q", path, state.Branch)
	case worktree.Rebasing:
		Git(t, params.Main, "worktree", "add", "-q", "--detach", path, state.Branch)
		rebaseMerge := filepath.Join(Output(t, path, "rev-parse", "--absolute-git-dir"), "rebase-merge")
		if err := os.MkdirAll(rebaseMerge, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(rebaseMerge, "head-name"), []byte("refs/heads/"+state.Branch+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	case worktree.Detached, worktree.Bare:
		Git(t, params.Main, "worktree", "add", "-q", "--detach", path, params.Root)
	}
	if slices.Contains(params.Seed.Gone, path) {
		if err := os.RemoveAll(path); err != nil {
			t.Fatal(err)
		}
	}
}

func writeFile(t testing.TB, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func remoteRef(name string) string {
	return "refs/remotes/" + origin + "/" + name
}
