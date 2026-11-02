package teardown_test

import (
	"context"
	"errors"
	"slices"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/storetest"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
	"github.com/LucasPcq/wtm/next/internal/service/teardown"
)

const (
	command = "sweep" // the command asking, whose codes teardown words
	plain   = "feat/plain"
	parent  = "feat/parent"
	child   = "feat/child"
	local   = "feat/local"   // its commit only feat/stacked holds
	stacked = "feat/stacked" // feat/local's commit, then one no other branch holds
	alone   = "feat/alone"   // a branch no worktree holds
)

var errRefused = errors.New("refused")

// world is a repository with a worktree per state a removal decides on, its records, and the service over them.
type world struct {
	git     *gittest.Fake
	store   *storetest.Fake
	daemon  *daemon
	queue   *queue
	service teardown.Service
	hooks   *[]env.HookChain
	purged  *[]string
	run     project.Run
}

type seed struct {
	Gone      []string // worktrees whose directory was deleted behind git's back
	Fail      map[string]error
	StoreFail map[string]error
	RunHooks  func(context.Context, env.HookChain) error
	Run       project.Run // run.toml
	Listed    []run.Job   // the jobs the run daemon lists
	Owed      []env.OwedDrop
}

func at(branch string) string {
	return "/repo/wt-" + worktree.PathSegment(branch)
}

func newWorld(s seed) world {
	git := gittest.NewFake(gittest.FakeParams{
		Main:   "/repo/main",
		Branch: "main",
		Local:  []string{alone},
		Worktrees: []worktree.Worktree{
			{Path: at(plain), State: worktree.OnBranch{Branch: plain}},
			{Path: at(parent), State: worktree.OnBranch{Branch: parent}},
			{Path: at(child), State: worktree.OnBranch{Branch: child}},
			{Path: at(local), State: worktree.OnBranch{Branch: local}},
			{Path: at(stacked), State: worktree.OnBranch{Branch: stacked}},
		},
		Commits: map[worktree.Ref][]string{
			worktree.LocalBranch{Name: local}:   {"local work"},
			worktree.LocalBranch{Name: stacked}: {"local work", "stacked work"},
		},
		Gone: s.Gone,
		Fail: s.Fail,
	})
	store := storetest.NewFake(storetest.FakeParams{Meta: map[string]worktree.Meta{
		plain:  {SourceBranch: "main", Ordinal: 1, Namespaces: []string{"db"}},
		parent: {SourceBranch: "main"},
		child:  {SourceBranch: parent},
		alone:  {SourceBranch: "main"},
	}, Fail: s.StoreFail})
	hooks := &[]env.HookChain{}
	runHooks := s.RunHooks
	if runHooks == nil {
		runHooks = func(_ context.Context, chain env.HookChain) error {
			*hooks = append(*hooks, chain)
			return nil
		}
	}
	d := &daemon{listed: slices.Clone(s.Listed)}
	q := &queue{owed: slices.Clone(s.Owed)}
	purged := &[]string{}
	return world{
		git:    git,
		store:  store,
		daemon: d,
		queue:  q,
		hooks:  hooks,
		purged: purged,
		run:    s.Run,
		service: teardown.Service{
			Git: git, Meta: store, EnvFiles: store, Daemon: d, Owed: q, StateDir: "/state", CommonDir: "/repo/main/.git",
			Gone: repository.GoneIn(git), RunHooks: runHooks, Shield: func(ctx context.Context) (context.Context, func()) { return ctx, func() {} },
			PurgeLogs: func(dir string) error { *purged = append(*purged, dir); return nil },
		},
	}
}

// sharedDB is a run.toml with a shared service whose namespace is named after the worktree.
func sharedDB() project.Run {
	return project.Run{Jobs: []project.Job{{Name: "db", Kind: project.Service, Cmd: "postgres", Scope: project.Shared, Ports: map[string]int{"PGPORT": 5432}, Namespace: &project.Namespace{Name: "acme_{worktree}", Create: "createdb", Remove: "dropdb"}}}}
}

// up is the shared db up in the main checkout, where it runs, and a worktree's claim on it.
func up(claimer string) []run.Job {
	return []run.Job{
		{Name: "db", WorkDir: "/repo/main", Up: true, Hold: run.HoldOwn, SharedDir: "/repo/main"},
		{Name: "db", WorkDir: at(claimer), Up: true, Hold: run.HoldClaim, SharedDir: "/repo/main"},
	}
}

// A daemon is the run daemon a test scripts: the jobs it lists, those a stop leaves up, why it cannot list or start, and every start and stop it got.
type daemon struct {
	listed   []run.Job
	stuck    []string
	jobsErr  error
	startErr error
	stopErr  error
	calls    []string
}

func (d *daemon) Jobs(context.Context) ([]run.Job, error) {
	return slices.Clone(d.listed), d.jobsErr
}

func (d *daemon) Start(_ context.Context, spec run.Spec, _ func(run.Line)) (run.Job, error) {
	d.calls = append(d.calls, "start "+spec.Job.Name+" "+spec.WorkDir)
	if d.startErr != nil {
		return run.Job{}, d.startErr
	}
	job := run.Job{Name: spec.Job.Name, WorkDir: spec.WorkDir, Up: true, Hold: run.HoldOwn}
	d.listed = append(d.listed, job)
	return job, nil
}

func (d *daemon) Stop(_ context.Context, req run.Stop) (run.Outcome, error) {
	d.calls = append(d.calls, "stop "+req.Name+" "+req.WorkDir)
	if slices.Contains(d.stuck, req.Name) {
		return run.OutcomeStopped, nil
	}
	d.listed = slices.DeleteFunc(d.listed, func(job run.Job) bool { return job.Name == req.Name && job.WorkDir == req.WorkDir })
	return run.OutcomeStopped, nil
}

func (d *daemon) StopAll(_ context.Context, req run.StopAll) ([]run.StoppedJob, error) {
	d.calls = append(d.calls, "stop_all "+req.WorkDir)
	if d.stopErr != nil {
		return nil, d.stopErr
	}
	d.listed = slices.DeleteFunc(d.listed, func(job run.Job) bool { return job.WorkDir == req.WorkDir })
	return nil, nil
}

// A queue is pending-removals.toml in memory.
type queue struct {
	owed []env.OwedDrop
}

func (q *queue) Load() ([]env.OwedDrop, error) {
	return slices.Clone(q.owed), nil
}

func (q *queue) Update(_ context.Context, change func([]env.OwedDrop) []env.OwedDrop) error {
	q.owed = change(slices.Clone(q.owed))
	return nil
}

// leaving removes the worktree from git's list but fails as git does when files it cannot delete stay on disk.
type leaving struct {
	*gittest.Fake
}

func (l leaving) RemoveWorktree(ctx context.Context, path string) error {
	if err := l.Fake.RemoveWorktree(ctx, path); err != nil {
		return err
	}
	return errRefused
}

func hookFails(context.Context, env.HookChain) error {
	return &env.HookFailure{Cmd: "make down", Err: errRefused}
}

// onClean is a config whose on_clean hook is make down.
func onClean() project.Config {
	return project.Config{Worktrees: project.Worktrees{BaseBranch: "main"}, OnClean: []project.Hook{{Cmd: "make down"}}}
}
