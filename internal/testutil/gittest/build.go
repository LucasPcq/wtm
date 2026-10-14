package gittest

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra/git"
)

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

	for _, branch := range localBranches(params) {
		Git(t, main, "update-ref", domain.LocalRefPrefix+branch, commits.tip(params.Commits[git.LocalBranch{Name: branch}]))
	}
	Git(t, main, "reset", "-q", "--hard")
	buildRemote(t, buildRemoteParams{Main: main, Commits: commits, Seed: params})

	for _, worktree := range params.Worktrees {
		buildWorktree(t, buildWorktreeParams{Main: main, Root: commits.made[Root], Worktree: worktree})
	}
	for _, dirty := range params.Dirty {
		writeFile(t, filepath.Join(dirty, "untracked"))
	}
	for _, folder := range params.Folders {
		writeFile(t, filepath.Join(folder, "work"))
	}
	dir := params.Dir
	if dir == "" {
		dir = main
	}
	return git.Repo{Dir: dir, Remote: domain.OriginRemote}
}

type builder struct {
	t    testing.TB
	dir  string
	made map[string]string
}

// tip makes each commit of the history it has not made yet, on the one before.
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
	branches := append([]string{params.Branch}, params.Local...)
	for _, worktree := range params.Worktrees {
		switch state := worktree.State.(type) {
		case git.OnBranch:
			branches = append(branches, state.Branch)
		case git.Rebasing:
			branches = append(branches, state.Branch)
		case git.Detached, git.Bare:
		}
	}
	slices.Sort(branches)
	return slices.Compact(branches)
}

type buildRemoteParams struct {
	Main    string
	Commits builder
	Seed    FakeParams
}

// buildRemote pushes what origin holds, then sets the remote branches to what
// was fetched of it, Origin's commits left out.
func buildRemote(t testing.TB, params buildRemoteParams) {
	t.Helper()
	origin := t.TempDir()
	Git(t, origin, "init", "-q", "--bare", "-b", params.Seed.Branch)
	Git(t, params.Main, "remote", "add", domain.OriginRemote, origin)
	for _, branch := range params.Seed.Remote {
		Git(t, params.Main, "push", "-q", domain.OriginRemote, params.Commits.tip(params.Seed.Commits[git.RemoteBranch{Name: branch}])+":"+domain.LocalRefPrefix+branch)
	}
	for branch, commits := range params.Seed.Origin {
		Git(t, params.Main, "push", "-q", "-f", domain.OriginRemote, params.Commits.tip(commits)+":"+domain.LocalRefPrefix+branch)
	}
	for branch := range params.Seed.Origin {
		Git(t, params.Main, "update-ref", "-d", remoteRef(branch))
	}
	for _, branch := range params.Seed.Remote {
		Git(t, params.Main, "update-ref", remoteRef(branch), params.Commits.tip(params.Seed.Commits[git.RemoteBranch{Name: branch}]))
	}
}

type buildWorktreeParams struct {
	Main     string
	Root     string
	Worktree git.Worktree
}

// buildWorktree leaves a rebase in progress as git does: HEAD detached, the
// branch it moves in the admin directory's rebase-merge/head-name.
func buildWorktree(t testing.TB, params buildWorktreeParams) {
	t.Helper()
	path := params.Worktree.Path
	switch state := params.Worktree.State.(type) {
	case git.OnBranch:
		Git(t, params.Main, "worktree", "add", "-q", path, state.Branch)
	case git.Rebasing:
		Git(t, params.Main, "worktree", "add", "-q", "--detach", path, state.Branch)
		rebaseMerge := filepath.Join(Output(t, path, "rev-parse", "--absolute-git-dir"), "rebase-merge")
		if err := os.MkdirAll(rebaseMerge, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(rebaseMerge, "head-name"), []byte(domain.LocalRefPrefix+state.Branch+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	case git.Detached, git.Bare:
		Git(t, params.Main, "worktree", "add", "-q", "--detach", path, params.Root)
	}
	if params.Worktree.Locked {
		Git(t, params.Main, "worktree", "lock", path)
	}
	if params.Worktree.Prunable {
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

func remoteRef(branch string) string {
	return "refs/remotes/" + domain.OriginRemote + "/" + branch
}
