// Package gittest gives tests a git repository, real or in memory, seeded from
// one description: Build makes it with git, NewFake simulates it in memory, so
// a test can run against both and check they agree.
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
	// Root is the first commit of every seeded repository, the base every
	// branch's commits start from; in the fake a commit is its message.
	Root = "root"

	origin = "origin"
)

// InitRepo creates a repository on main with one empty commit in a temporary
// directory and returns it.
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

// Git runs git with args in dir and fails the test if it fails.
func Git(t testing.TB, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %s: %v", strings.Join(args, " "), out, err)
	}
}

// Output runs git with args in dir and returns its trimmed stdout, failing the
// test if it fails.
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

// CreateBranch creates the branch name at dir's HEAD.
func CreateBranch(t testing.TB, dir, name string) {
	t.Helper()
	Git(t, dir, "branch", name)
}

// AddOrigin gives the repository at dir a bare origin with main pushed to it,
// and returns the origin's directory.
func AddOrigin(t testing.TB, dir string) string {
	t.Helper()
	remote := t.TempDir()
	Git(t, remote, "init", "--bare", "-b", "main")
	Git(t, dir, "remote", "add", origin, remote)
	Git(t, dir, "push", "-u", origin, "main")
	return remote
}

// PushBranch pushes the branch name to origin, tracking it.
func PushBranch(t testing.TB, dir, name string) {
	t.Helper()
	Git(t, dir, "push", "-u", origin, name)
}

// TempDir returns a temporary directory with its symlinks resolved, as git
// spells the paths it lists (macOS's /var is a symlink).
func TempDir(t testing.TB) string {
	t.Helper()
	return Resolved(t, t.TempDir())
}

// Resolved returns path with its symlinks resolved, failing the test when it
// does not exist.
func Resolved(t testing.TB, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}

	return resolved
}

// Symlinked returns a path that is a symlink to a temporary directory, to
// check that paths are reported resolved.
func Symlinked(t testing.TB) string {
	t.Helper()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(t.TempDir(), link); err != nil {
		t.Fatal(err)
	}

	return link
}

// Names returns the name of each of branches.
func Names(branches []worktree.Branch) []string {
	names := make([]string, 0, len(branches))
	for _, b := range branches {
		names = append(names, b.Name)
	}
	return names
}

// State is what a repository lists: its worktrees and its branches, without
// their tips, which differ between git and the fake.
type State struct {
	Worktrees []worktree.Worktree
	Branches  worktree.Branches
}

// reader is what StateOf reads, implemented by git.Repo and Fake alike.
type reader interface {
	Worktrees(ctx context.Context) ([]worktree.Worktree, error)
	Branches(ctx context.Context) (worktree.Branches, error)
}

// StateOf reads repo's State, failing the test when it cannot; commits are named by the fake and hashed by git, so neither a worktree's head nor a branch's tip is kept.
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

	for i := range worktrees {
		worktrees[i].Head = ""
	}
	return State{Worktrees: worktrees, Branches: worktree.Branches{Local: untipped(branches.Local), Remote: untipped(branches.Remote)}}
}

// untipped clears the tip of each of branches, in place.
func untipped(branches []worktree.Branch) []worktree.Branch {
	for i := range branches {
		branches[i].Tip = ""
	}
	return branches
}

// Build makes the repository params describes with git: the main worktree at
// params.Main, its branches and their commits, a bare origin, the linked
// worktrees, and the dirty, taken and empty paths. Dir, Fail, Changes and
// Conflicts are the fake's alone and are ignored. It returns the git.Repo run
// from the main worktree.
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

	buildRemote(t, main, commits, params)
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

	return git.Repo{Dir: main}
}

// builder makes the commits of a seed in the repository at dir, each once:
// made maps a commit's message to its hash.
type builder struct {
	t    testing.TB
	dir  string
	made map[string]string
}

// tip makes commits as a chain of empty commits on Root, reusing any already
// made, and returns the last one's hash.
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

// localBranches returns the local branches a seed implies, sorted and each
// once: its main branch, its Local and those its worktrees are on.
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

// buildRemote gives the repository a bare origin holding seed's Remote and
// Origin branches, then sets the remote-tracking branches to seed's Remote
// alone, so an Origin branch is on origin but not fetched yet.
func buildRemote(t testing.TB, main string, built builder, seed FakeParams) {
	t.Helper()
	bare := t.TempDir()
	Git(t, bare, "init", "-q", "--bare", "-b", seed.Branch)
	Git(t, main, "remote", "add", origin, bare)

	for _, name := range seed.Remote {
		Git(t, main, "push", "-q", origin, built.tip(seed.Commits[worktree.RemoteBranch{Name: name}])+":refs/heads/"+name)
	}

	for name, commits := range seed.Origin {
		Git(t, main, "push", "-q", "-f", origin, built.tip(commits)+":refs/heads/"+name)
	}

	for name := range seed.Origin {
		Git(t, main, "update-ref", "-d", remoteRef(name))
	}

	for _, name := range seed.Remote {
		Git(t, main, "update-ref", remoteRef(name), built.tip(seed.Commits[worktree.RemoteBranch{Name: name}]))
	}
}

// midwayMarks are the files git keeps while it is in the middle of an operation on a worktree's checkout, by operation.
var midwayMarks = map[worktree.Operation]string{
	worktree.OperationMerge:      "MERGE_HEAD",
	worktree.OperationCherryPick: "CHERRY_PICK_HEAD",
	worktree.OperationRevert:     "REVERT_HEAD",
	worktree.OperationApply:      filepath.Join("rebase-apply", "applying"),
}

// buildWorktreeParams is one linked worktree to build in the repository at
// Main; Root is the hash a detached worktree points at.
type buildWorktreeParams struct {
	Main     string
	Root     string
	Worktree worktree.Worktree
	Seed     FakeParams
}

// buildWorktree adds the worktree in its seeded state, a rebase faked by its
// rebase-merge state, a bisect by its BISECT_START and an operation midway by
// the file git keeps for it (midwayMarks), locks it when seeded locked, then deletes its directory when the
// seed lists it gone.
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
	case worktree.Bisecting:
		Git(t, params.Main, "worktree", "add", "-q", "--detach", path, state.Branch)
		bisectStart := filepath.Join(Output(t, path, "rev-parse", "--absolute-git-dir"), "BISECT_START")
		if err := os.WriteFile(bisectStart, []byte(state.Branch+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	case worktree.Detached, worktree.Bare:
		Git(t, params.Main, "worktree", "add", "-q", "--detach", path, params.Root)
	}

	if mark, ok := midwayMarks[params.Worktree.Midway]; ok {
		marked := filepath.Join(Output(t, path, "rev-parse", "--absolute-git-dir"), mark)
		if err := os.MkdirAll(filepath.Dir(marked), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(marked, []byte(Output(t, path, "rev-parse", "HEAD")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if params.Worktree.Locked {
		Git(t, params.Main, "worktree", "lock", path)
	}

	if slices.Contains(params.Seed.Gone, path) {
		if err := os.RemoveAll(path); err != nil {
			t.Fatal(err)
		}
	}
}

// writeFile writes a small file at path, creating its directories.
func writeFile(t testing.TB, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte("work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// remoteRef is the remote-tracking ref of origin's branch name.
func remoteRef(name string) string {
	return "refs/remotes/" + origin + "/" + name
}
