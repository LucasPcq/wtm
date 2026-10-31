package runup_test

import (
	"context"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/config"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/oplock"
	"github.com/LucasPcq/wtm/next/internal/adapter/store"
	"github.com/LucasPcq/wtm/next/internal/command/runup"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

const (
	featA       = "feat/a"
	featB       = "feat/b"
	verbatim    = "feat/v"
	fresh       = "feat/fresh"
	gone        = "feat/gone"
	projectToml = "[worktrees]\nbase_path = \"../.trees\"\nbase_branch = \"main\"\n"
	// runToml declares two services on one port name, a task that changes the shared service's data, a shared service with a namespace, a runner, and profiles: dev the default.
	runToml = `
[[job]]
name = "api"
kind = "service"
cmd = "api"
ports = { PORT = 4000 }
url = { port = "PORT" }

[[job]]
name = "web"
kind = "service"
cmd = "web"
ports = { PORT = 3000 }
url = { port = "PORT" }

[[job]]
name = "migrate"
kind = "task"
cmd = "migrate"
touches = ["db"]

[[job]]
name = "db"
kind = "service"
cmd = "db"
scope = "shared"
ports = { DB_PORT = 5432 }

[job.namespace]
name = "app_{worktree}"
create = "createdb"
remove = "dropdb"

[[job]]
name = "turbo"
kind = "service"
cmd = "turbo dev"
runs = ["web"]

[[profile]]
name = "dev"
jobs = ["api", "web"]
default = true

[[profile]]
name = "full"
jobs = ["migrate", "db", "api", "web"]

[[profile]]
name = "twice"
jobs = ["turbo", "web"]
`
)

var startedAt = time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)

type world struct {
	root     string
	main     string
	stateDir string
	meta     store.Meta
	daemon   *daemon
	silent   []int
	usecase  runup.UseCase
}

// newWorld is a repository with worktrees in every state run up decides on: two numbered ones, a verbatim one sharing main's ports, one no record numbers yet, and one whose directory is gone; run.toml declares runToml.
func newWorld(t *testing.T) *world {
	t.Helper()
	root := gittest.TempDir(t)
	main := filepath.Join(root, "main")
	repo := gittest.Build(t, gittest.FakeParams{
		Main:   main,
		Branch: "main",
		Worktrees: []worktree.Worktree{
			{Path: linkedPath(root, featA), State: worktree.OnBranch{Branch: featA}},
			{Path: linkedPath(root, featB), State: worktree.OnBranch{Branch: featB}},
			{Path: linkedPath(root, verbatim), State: worktree.OnBranch{Branch: verbatim}},
			{Path: linkedPath(root, fresh), State: worktree.OnBranch{Branch: fresh}},
			{Path: linkedPath(root, gone), State: worktree.OnBranch{Branch: gone}},
		},
		Gone: []string{linkedPath(root, gone)},
	})
	stateDir := filepath.Join(main, ".git", "wtm")
	writeFile(t, filepath.Join(stateDir, "config.toml"), projectToml)
	writeFile(t, filepath.Join(stateDir, "run.toml"), runToml)
	meta := store.Meta{StateDir: stateDir}
	w := &world{root: root, main: main, stateDir: stateDir, meta: meta, daemon: &daemon{}}
	w.record(t, featA, worktree.Meta{SourceBranch: "main", Ordinal: 1, Isolation: project.Isolated})
	w.record(t, featB, worktree.Meta{SourceBranch: "main", Ordinal: 2, Isolation: project.Isolated})
	w.record(t, verbatim, worktree.Meta{SourceBranch: "main", Isolation: project.Verbatim})

	ids := 0
	w.usecase = runup.UseCase{
		Git:      repo,
		Meta:     meta,
		EnvFiles: store.EnvFiles{},
		Daemon:   w.daemon,
		Listening: func(_ context.Context, ports []int, _ time.Duration) map[int]bool {
			answered := map[int]bool{}
			for _, port := range ports {
				answered[port] = !slices.Contains(w.silent, port)
			}
			return answered
		},
		StateDir:   stateDir,
		CommonDir:  filepath.Join(main, ".git"),
		Repository: repository.Service{Config: config.Project{StateDir: stateDir, GlobalPath: filepath.Join(root, "no-global.toml")}, Git: repo, MainPath: main, Restart: &domain.FollowUp{Command: "run daemon restart"}},
		Operations: operation.Service{
			Locker:    locks{oplock.Locks{StateDir: stateDir}},
			Publisher: &operationtest.Recorder{},
			NewOpID: func() string {
				ids++
				return "op-" + string(rune('0'+ids))
			},
			Now: func() time.Time { return startedAt },
		},
		Provision: provision.Service{Git: repo, Meta: meta},
	}
	return w
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

// A daemon is the run daemon a test scripts: the jobs it lists, those it refuses or reports crashed, the lines a start prints, whether it is of another build, and every start and stop it got.
type daemon struct {
	mu        sync.Mutex
	listed    []run.Job
	refuse    map[string]error    // by worktree path and job name, joined by a space
	crash     map[string]bool     // likewise: listed crashed once started
	lines     map[string][]string // likewise: what its start prints
	skewed    *run.Skewed
	ensureErr error
	onStart   func(spec run.Spec) // runs as a start begins
	started   []run.Spec
	stopped   []string
}

func key(workDir, name string) string {
	return workDir + " " + name
}

func (d *daemon) Jobs(context.Context) ([]run.Job, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.skewed != nil {
		return slices.Clone(d.listed), d.skewed
	}
	return slices.Clone(d.listed), nil
}

func (d *daemon) Ensure(context.Context) error {
	return d.ensureErr
}

func (d *daemon) Start(_ context.Context, spec run.Spec, out func(run.Line)) (run.Job, error) {
	if d.onStart != nil {
		d.onStart(spec)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.started = append(d.started, spec)
	at := key(spec.WorkDir, spec.Job.Name)
	for _, line := range d.lines[at] {
		out(run.Line{At: startedAt, Text: line})
	}
	if err := d.refuse[at]; err != nil {
		return run.Job{}, err
	}
	job := run.Job{Name: spec.Job.Name, WorkDir: spec.WorkDir, Profile: spec.Profile, Up: spec.Job.Kind != project.Task, Kind: spec.Job.Kind, State: run.Running, URL: "http://" + spec.Job.Name + ".localhost"}
	if spec.Job.Kind == project.Task {
		job.State = run.Exited
	}
	listed := job
	if d.crash[at] {
		listed.Up, listed.State, listed.ExitCode, listed.LastLine = false, run.Crashed, new(1), &run.Line{At: startedAt, Text: "boom"}
	}
	d.listed = append(d.listed, listed)
	return job, nil
}

func (d *daemon) StopAll(_ context.Context, req run.StopAll) ([]run.StoppedJob, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.stopped = append(d.stopped, req.WorkDir)
	d.listed = slices.DeleteFunc(d.listed, func(job run.Job) bool { return job.WorkDir == req.WorkDir })
	return nil, nil
}
