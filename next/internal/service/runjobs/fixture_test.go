package runjobs_test

import (
	"context"
	"slices"
	"sync"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/dotenv"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

const (
	mainPath = "/repo/main"
	featA    = "feat/a"
	featPath = "/repo/wt-feat-a"
)

// repo is a repository of one worktree besides main, whose run.toml declares a service, api.
var repo = worktree.Repository{
	MainPath: mainPath,
	Run:      project.Run{Declared: true, Jobs: []project.Job{{Name: "api", Kind: project.Service, Cmd: "api"}}},
	Snapshot: worktree.Snapshot{Worktrees: []worktree.Worktree{
		{Path: mainPath, Main: true, State: worktree.OnBranch{Branch: "main"}},
		{Path: featPath, State: worktree.OnBranch{Branch: featA}},
	}},
}

// A daemon is wtm's daemon a test scripts: the jobs it lists, whether none answers with processes in its index (unknown) even once made sure of (staysUnknown), and every call it got, in order.
type daemon struct {
	mu           sync.Mutex
	listed       []run.Job
	unknown      bool
	staysUnknown bool
	ensured      bool
	calls        []string
	started      []run.Spec
	stopLines    []string
}

func (d *daemon) Jobs(context.Context) ([]run.Job, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.unknown && (!d.ensured || d.staysUnknown) {
		return nil, run.ErrUnknown
	}
	return slices.Clone(d.listed), nil
}

func (d *daemon) Ensure(context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.ensured = true
	d.calls = append(d.calls, "ensure")
	return nil
}

func (d *daemon) Start(_ context.Context, spec run.Spec, _ func(run.Line)) (run.Job, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = append(d.calls, "start "+spec.Job.Name)
	d.started = append(d.started, spec)
	job := run.Job{Name: spec.Job.Name, WorkDir: spec.WorkDir, Up: true, Kind: spec.Job.Kind, State: run.Running, Hold: run.HoldOwn}
	d.listed = append(slices.DeleteFunc(d.listed, func(j run.Job) bool { return j.Name == job.Name && j.WorkDir == job.WorkDir }), job)
	return job, nil
}

func (d *daemon) Stop(_ context.Context, req run.Stop, out func(run.Line)) (run.Outcome, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = append(d.calls, "stop "+req.Name)
	for _, line := range d.stopLines {
		out(run.Line{Text: line})
	}
	for i, job := range d.listed {
		if job.Name == req.Name && job.WorkDir == req.WorkDir {
			d.listed[i].Up, d.listed[i].State = false, run.Stopped
		}
	}
	return run.OutcomeStopped, nil
}

func (d *daemon) StopAll(context.Context, run.StopAll) ([]run.StoppedJob, error) { return nil, nil }

// noEnv is a worktree with no .env file.
type noEnv struct{}

func (noEnv) Read(string) ([]dotenv.Line, error) { return nil, nil }

// noMeta is a worktree whose record is never read or written by these tests.
type noMeta struct{}

func (noMeta) Load(string) (worktree.Meta, error) { return worktree.Meta{}, nil }

func (noMeta) Update(context.Context, worktree.MetaUpdate) (domain.Written, error) {
	return domain.Written{}, nil
}
