package clean_test

import (
	"context"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/config"
	"github.com/LucasPcq/wtm/next/internal/adapter/git"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/joblog"
	"github.com/LucasPcq/wtm/next/internal/adapter/oplock"
	"github.com/LucasPcq/wtm/next/internal/adapter/shell"
	"github.com/LucasPcq/wtm/next/internal/adapter/store"
	"github.com/LucasPcq/wtm/next/internal/command/clean"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
	"github.com/LucasPcq/wtm/next/internal/service/teardown"
)

const (
	plain       = "feat/plain"
	parent      = "feat/parent"
	child       = "feat/child"
	grandchild  = "feat/grandchild"
	dirty       = "feat/dirty"
	locked      = "feat/locked"
	pushed      = "feat/pushed"
	local       = "feat/local"
	projectToml = "[worktrees]\nbase_path = \"../.trees\"\nbase_branch = \"main\"\n"
	// runToml declares a shared service whose namespace, named after the worktree, a remove command drops by writing its name into the main checkout's dropped.txt.
	runToml = `
[[job]]
name = "db"
kind = "service"
cmd = "db"
scope = "shared"
ports = { DB_PORT = 5432 }

[job.namespace]
name = "app_{worktree}"
create = "createdb"
remove = "echo $WTM_NAMESPACE:$DB_PORT >> dropped.txt"
`
)

var createdAt = time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)

// seed is what a world holds beyond its worktrees: its config.toml, the jobs wtm's daemon runs, the pull requests the forge knows and why it cannot be asked.
type seed struct {
	Config string
	Jobs   []run.Job
	JobErr error
	PRs    []worktree.PullRequest
	PRErr  error
}

type world struct {
	root     string
	main     string
	stateDir string
	repo     git.Repo
	meta     store.Meta
	daemon   *daemon
	asked    *[]string // the branches the forge was asked about
	usecase  clean.UseCase
}

// newWorld is a repository with a worktree in every state clean decides on: plain, a parent with a child and a grandchild, uncommitted changes, locked, a branch origin has, a branch holding a commit no other one does, and one on no branch.
func newWorld(t *testing.T, s seed) world {
	t.Helper()
	root := gittest.TempDir(t)
	main := filepath.Join(root, "main")
	repo := gittest.Build(t, gittest.FakeParams{
		Main:   main,
		Branch: "main",
		Remote: []string{"main", pushed},
		Worktrees: []worktree.Worktree{
			{Path: linkedPath(root, plain), State: worktree.OnBranch{Branch: plain}},
			{Path: linkedPath(root, parent), State: worktree.OnBranch{Branch: parent}},
			{Path: linkedPath(root, child), State: worktree.OnBranch{Branch: child}},
			{Path: linkedPath(root, grandchild), State: worktree.OnBranch{Branch: grandchild}},
			{Path: linkedPath(root, dirty), State: worktree.OnBranch{Branch: dirty}},
			{Path: linkedPath(root, locked), State: worktree.OnBranch{Branch: locked}, Locked: true},
			{Path: linkedPath(root, pushed), State: worktree.OnBranch{Branch: pushed}},
			{Path: linkedPath(root, local), State: worktree.OnBranch{Branch: local}},
			{Path: linkedPath(root, "detached"), State: worktree.Detached{}},
		},
		Dirty: []string{linkedPath(root, dirty)},
		Commits: map[worktree.Ref][]string{
			worktree.LocalBranch{Name: local}:   {"local work"},
			worktree.LocalBranch{Name: pushed}:  {"pushed work"},
			worktree.RemoteBranch{Name: pushed}: {"pushed work"},
		},
	})
	stateDir := filepath.Join(main, ".git", "wtm")
	writeFile(t, filepath.Join(stateDir, "config.toml"), configOf(s))
	writeFile(t, filepath.Join(stateDir, "run.toml"), runToml)
	meta := store.Meta{StateDir: stateDir}
	d := &daemon{listed: slices.Clone(s.Jobs), jobsErr: s.JobErr}
	w := world{root: root, main: main, stateDir: stateDir, repo: repo, meta: meta, daemon: d, asked: new([]string)}
	for _, branch := range []string{plain, parent, dirty, locked, pushed, local} {
		w.record(t, branch, worktree.Meta{SourceBranch: "main", CreatedAt: createdAt.Format(time.RFC3339)})
	}
	w.record(t, child, worktree.Meta{SourceBranch: parent, Ordinal: 2})
	w.record(t, grandchild, worktree.Meta{SourceBranch: child})

	ids := 0
	w.usecase = clean.UseCase{
		Git:  repo,
		Meta: meta,
		Jobs: d.Jobs,
		PullRequests: func(_ context.Context, branches []string) ([]worktree.PullRequest, error) {
			*w.asked = append(*w.asked, branches...)
			return s.PRs, s.PRErr
		},
		Teardown: teardown.Service{
			Git: repo, Meta: meta, EnvFiles: store.EnvFiles{}, Daemon: d, Owed: store.Owed{StateDir: stateDir}, StateDir: stateDir, CommonDir: filepath.Join(main, ".git"),
			Gone: repository.GoneIn(repo), RunHooks: shell.RunHooks, PurgeLogs: joblog.Purge, Shield: shell.Shield,
		},
		Repository: repository.Service{Config: config.Project{StateDir: stateDir, GlobalPath: filepath.Join(root, "no-global.toml")}, Git: repo, MainPath: main},
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

// failingDelete removes worktrees as git does but refuses to delete any branch.
type failingDelete struct {
	git.Repo
}

func (failingDelete) DeleteBranch(context.Context, string) error {
	return errBranchHeld
}

// leavingFiles removes the worktree from git's list but fails as git does when files it cannot delete stay on disk.
type leavingFiles struct {
	git.Repo
}

func (l leavingFiles) RemoveWorktree(ctx context.Context, path string) error {
	if err := l.Repo.RemoveWorktree(ctx, path); err != nil {
		return err
	}
	return errPermission
}

// failingRemove refuses to remove the worktree at path, which git still lists, as it does a worktree it cannot remove.
type failingRemove struct {
	git.Repo
	path string
}

func (f failingRemove) RemoveWorktree(ctx context.Context, path string) error {
	if path == f.path {
		return errPermission
	}
	return f.Repo.RemoveWorktree(ctx, path)
}

// bare lists the main worktree as the entry of a bare repository, which has no working tree.
type bare struct {
	git.Repo
}

func (b bare) Snapshot(ctx context.Context) (worktree.Snapshot, error) {
	snapshot, err := b.Repo.Snapshot(ctx)
	if err == nil {
		snapshot.Worktrees[0].State = worktree.Bare{}
	}
	return snapshot, err
}

// broken fails to count what a removal would lose, as git does on a corrupt object.
type broken struct {
	git.Repo
}

func (broken) Unpushed(context.Context, worktree.Losing) (int, error) {
	return 0, errBroken
}

// newerRecord is a record a newer wtm wrote, at a schema this one does not know.
const newerRecord = `{"schema": 99, "source_branch": "main"}`

// A daemon is wtm's daemon a test scripts: the jobs it lists, those a stop leaves up, why it cannot list, and every start and stop it got.
type daemon struct {
	listed  []run.Job
	stuck   []string
	jobsErr error
	calls   []string
}

func (d *daemon) Jobs(context.Context) ([]run.Job, error) {
	return slices.Clone(d.listed), d.jobsErr
}

func (d *daemon) Start(_ context.Context, spec run.Spec, _ func(run.Line)) (run.Job, error) {
	d.calls = append(d.calls, "start "+spec.Job.Name)
	job := run.Job{Name: spec.Job.Name, WorkDir: spec.WorkDir, Up: true, Hold: run.HoldOwn}
	d.listed = append(d.listed, job)
	return job, nil
}

func (d *daemon) Stop(_ context.Context, req run.Stop, _ func(run.Line)) (run.Outcome, error) {
	d.calls = append(d.calls, "stop "+req.Name+" "+filepath.Base(req.WorkDir))
	if !slices.Contains(d.stuck, req.Name) {
		d.listed = slices.DeleteFunc(d.listed, func(job run.Job) bool { return job.Name == req.Name && job.WorkDir == req.WorkDir })
	}
	return run.OutcomeStopped, nil
}

func (d *daemon) StopAll(_ context.Context, req run.StopAll) ([]run.StoppedJob, error) {
	d.calls = append(d.calls, "stop_all "+filepath.Base(req.WorkDir))
	d.listed = slices.DeleteFunc(d.listed, func(job run.Job) bool { return job.WorkDir == req.WorkDir })
	return nil, nil
}

// dbUp is the shared db up in the main checkout of the world at main, and the claims of the worktrees at claimers on it.
func dbUp(main string, claimers ...string) []run.Job {
	jobs := []run.Job{{Name: "db", WorkDir: main, Up: true, Hold: run.HoldOwn, SharedDir: main}}
	for _, claimer := range claimers {
		jobs = append(jobs, run.Job{Name: "db", WorkDir: claimer, Up: true, Hold: run.HoldClaim, SharedDir: main})
	}
	return jobs
}
