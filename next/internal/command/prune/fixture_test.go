package prune_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/config"
	"github.com/LucasPcq/wtm/next/internal/adapter/git"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/oplock"
	"github.com/LucasPcq/wtm/next/internal/adapter/shell"
	"github.com/LucasPcq/wtm/next/internal/adapter/store"
	"github.com/LucasPcq/wtm/next/internal/command/prune"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
	"github.com/LucasPcq/wtm/next/internal/service/teardown"
)

const (
	merged      = "feat/merged"    // its pull request merged at its tip: done with, safe
	child       = "feat/child"     // created from feat/merged
	closed      = "feat/closed"    // its pull request closed at its tip, its commit on origin: done with, safe
	moved       = "feat/moved"     // its pull request merged, a commit made since: not done with
	gone        = "feat/gone"      // tracking a branch origin deleted, no commit of its own: done with, safe
	goneWork    = "feat/gone-work" // tracking a branch origin deleted, holding a commit no other branch holds: done with, unsafe
	dirty       = "feat/dirty"     // its pull request merged at its tip, uncommitted changes: done with, unsafe
	open        = "feat/open"      // its pull request open: not done with
	plain       = "feat/plain"     // no pull request: not done with
	projectToml = "[worktrees]\nbase_path = \"../.trees\"\nbase_branch = \"main\"\n"
)

var (
	createdAt  = time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	errOffline = errors.New("offline")
	errBroken  = errors.New("bad ref")
)

// seed is what a world holds beyond its worktrees: its config.toml, the jobs the run daemon runs, and why the forge cannot be asked.
type seed struct {
	Config string
	Jobs   []run.Job
	PRErr  error
}

type world struct {
	root     string
	main     string
	stateDir string
	repo     git.Repo
	meta     store.Meta
	prs      []worktree.PullRequest
	usecase  prune.UseCase
}

// newWorld is a repository with a worktree in every state prune decides on, and the pull requests the forge knows of them.
func newWorld(t *testing.T, s seed) world {
	t.Helper()
	root := gittest.TempDir(t)
	main := filepath.Join(root, "main")
	on := func(branch string) worktree.Worktree {
		return worktree.Worktree{Path: linkedPath(root, branch), State: worktree.OnBranch{Branch: branch}}
	}
	repo := gittest.Build(t, gittest.FakeParams{
		Main:      main,
		Branch:    "main",
		Remote:    []string{"main", closed, gone, goneWork},
		Tracking:  []string{gone, goneWork},
		Deleted:   []string{gone, goneWork},
		Worktrees: []worktree.Worktree{on(merged), on(child), on(closed), on(moved), on(gone), on(goneWork), on(dirty), on(open), on(plain)},
		Dirty:     []string{linkedPath(root, dirty)},
		Commits: map[worktree.Ref][]string{
			worktree.LocalBranch{Name: merged}:    {"merged work"},
			worktree.LocalBranch{Name: closed}:    {"closed work"},
			worktree.RemoteBranch{Name: closed}:   {"closed work"},
			worktree.LocalBranch{Name: moved}:     {"moved work", "after the merge"},
			worktree.LocalBranch{Name: goneWork}:  {"gone work"},
			worktree.RemoteBranch{Name: goneWork}: {"gone work"},
			worktree.LocalBranch{Name: dirty}:     {"dirty work"},
			worktree.LocalBranch{Name: open}:      {"open work"},
		},
	})
	stateDir := filepath.Join(main, ".git", "wtm")
	writeFile(t, filepath.Join(stateDir, "config.toml"), configOf(s))
	meta := store.Meta{StateDir: stateDir}
	w := world{root: root, main: main, stateDir: stateDir, repo: repo, meta: meta}
	for _, branch := range []string{merged, closed, moved, gone, goneWork, dirty, open, plain} {
		w.record(t, branch, worktree.Meta{SourceBranch: "main", CreatedAt: createdAt.Format(time.RFC3339)})
	}
	w.record(t, child, worktree.Meta{SourceBranch: merged})

	tip := func(ref string) string { return gittest.Output(t, main, "rev-parse", ref) }
	w.prs = []worktree.PullRequest{
		{Number: 1, URL: "https://github.com/acme/web/pull/1", Branch: merged, State: worktree.PullRequestMerged, Head: tip(merged)},
		{Number: 2, URL: "https://github.com/acme/web/pull/2", Branch: closed, State: worktree.PullRequestClosed, Head: tip(closed)},
		{Number: 3, URL: "https://github.com/acme/web/pull/3", Branch: moved, State: worktree.PullRequestMerged, Head: tip(moved + "~1")},
		{Number: 4, URL: "https://github.com/acme/web/pull/4", Branch: dirty, State: worktree.PullRequestMerged, Head: tip(dirty)},
		{Number: 5, URL: "https://github.com/acme/web/pull/5", Branch: open, State: worktree.PullRequestOpen, Head: tip(open)},
	}

	ids := 0
	w.usecase = prune.UseCase{
		Git:          repo,
		Meta:         meta,
		Jobs:         func(context.Context) ([]run.Job, error) { return s.Jobs, nil },
		PullRequests: func(context.Context, []string) ([]worktree.PullRequest, error) { return w.prs, s.PRErr },
		Teardown:     teardown.Service{Git: repo, Meta: meta, EnvFiles: store.EnvFiles{}, StateDir: stateDir, RunHooks: shell.RunHooks, Shield: shell.Shield},
		Repository:   repository.Service{Config: config.Project{StateDir: stateDir, GlobalPath: filepath.Join(root, "no-global.toml")}, Git: repo, MainPath: main},
		Operations: operation.Service{
			Locker:    locks{oplock.Locks{StateDir: stateDir}},
			Publisher: &operationtest.Recorder{},
			NewOpID: func() string {
				ids++
				return "op-" + string(rune('0'+ids))
			},
			Now: func() time.Time { return createdAt },
		},
	}
	if s.PRErr != nil {
		w.usecase.PullRequests = func(context.Context, []string) ([]worktree.PullRequest, error) { return nil, s.PRErr }
	}
	return w
}

func configOf(s seed) string {
	if s.Config != "" {
		return s.Config
	}
	return projectToml
}

func linkedPath(root, name string) string {
	return filepath.Join(root, "wt-"+worktree.PathSegment(name))
}

type locks struct {
	oplock.Locks
}

func (l locks) Take(op domain.Operation) (operation.Hold, error) {
	return l.Locks.Take(op)
}

// offline is a repository whose origin cannot be fetched.
type offline struct {
	git.Repo
}

func (offline) RefreshOrigin(context.Context, []string) error {
	return errOffline
}

// noFetch is a repository that records whether origin was fetched.
type noFetch struct {
	git.Repo
	fetched *bool
}

func (n noFetch) RefreshOrigin(context.Context, []string) error {
	*n.fetched = true
	return nil
}

// broken fails to read the branches origin deleted, as git does on a corrupt ref.
type broken struct {
	git.Repo
}

func (broken) Gone(context.Context) (map[string]bool, error) {
	return nil, errBroken
}
