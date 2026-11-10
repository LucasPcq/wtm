package runstop_test

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
	"github.com/LucasPcq/wtm/next/internal/command/runstop"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
	"github.com/LucasPcq/wtm/next/internal/service/runjobs"
)

const (
	featA       = "feat/a"
	featB       = "feat/b"
	projectToml = "[worktrees]\nbase_path = \"../.trees\"\nbase_branch = \"main\"\n"
	runToml     = `
[[job]]
name = "api"
kind = "service"
cmd = "api"

[[job]]
name = "web"
kind = "service"
cmd = "web"

[[job]]
name = "cache"
kind = "service"
cmd = "docker compose up -d cache"
stop = "docker compose down"

[[job]]
name = "db"
kind = "service"
cmd = "db"
scope = "shared"

[[profile]]
name = "dev"
jobs = ["api", "web"]
default = true

[[profile]]
name = "full"
jobs = ["db", "cache", "api", "web"]
`
)

var startedAt = time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)

type world struct {
	root     string
	main     string
	stateDir string
	daemon   *daemon
	usecase  runstop.UseCase
}

// newWorld is a repository with two worktrees and main, run.toml declaring runToml, and a daemon running nothing yet.
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
		},
	})
	stateDir := filepath.Join(main, ".git", "wtm")
	writeFile(t, filepath.Join(stateDir, "config.toml"), projectToml)
	writeFile(t, filepath.Join(stateDir, "run.toml"), runToml)
	w := &world{root: root, main: main, stateDir: stateDir, daemon: &daemon{}}

	ids := 0
	w.usecase = runstop.UseCase{
		Stops:      runjobs.Stops{Daemon: w.daemon, Gone: repository.GoneIn(repo), Located: repository.LocatedIn(repo), Locate: repository.LocateIn(repo)},
		Repository: repository.Service{Config: config.Project{StateDir: stateDir, GlobalPath: filepath.Join(root, "no-global.toml")}, Git: repo, MainPath: main, Restart: &domain.FollowUp{Command: "daemon restart"}},
		Operations: operation.Service{
			Locker:    locks{oplock.Locks{StateDir: stateDir}},
			Publisher: &operationtest.Recorder{},
			NewOpID: func() string {
				ids++
				return "op-" + string(rune('0'+ids))
			},
			Now: func() time.Time { return startedAt },
		},
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

// A daemon is wtm's daemon a test scripts: the jobs it lists, the stops it refuses, what it says of a stop, whether none answers with processes in its index (unknown) until it is made sure of, and every stop it got.
type daemon struct {
	mu        sync.Mutex
	listed    []run.Job
	refuse    map[string]error       // by worktree path and job name, joined by a space
	outcomes  map[string]run.Outcome // likewise; stopped by default
	skewed    *run.Skewed
	jobsErr   error // what reading the jobs fails on
	ensureErr error // what making sure of a daemon fails on
	unknown   bool
	ensured   bool
	// staysUnknown keeps the jobs unknown once made sure of: a daemon started that still cannot tell
	staysUnknown bool
	onStop       func()
	stopped      []string
}

func key(workDir, name string) string {
	return workDir + " " + name
}

func (d *daemon) Jobs(context.Context) ([]run.Job, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.jobsErr != nil {
		return nil, d.jobsErr
	}
	if d.unknown && (!d.ensured || d.staysUnknown) {
		return nil, run.ErrUnknown
	}
	if d.skewed != nil {
		return slices.Clone(d.listed), d.skewed
	}
	return slices.Clone(d.listed), nil
}

func (d *daemon) Ensure(context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.ensured = true
	return d.ensureErr
}

func (d *daemon) Stop(_ context.Context, req run.Stop, _ func(run.Line)) (run.Outcome, error) {
	if d.onStop != nil {
		d.onStop()
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	at := key(req.WorkDir, req.Name)
	d.stopped = append(d.stopped, at)
	if err := d.refuse[at]; err != nil {
		return "", err
	}
	d.listed = slices.DeleteFunc(d.listed, func(job run.Job) bool { return job.WorkDir == req.WorkDir && job.Name == req.Name })
	if outcome, ok := d.outcomes[at]; ok {
		return outcome, nil
	}
	return run.OutcomeStopped, nil
}

// service is a job of name a worktree runs for itself, up, started under profile.
func service(name, dir, profile string) run.Job {
	return run.Job{Name: name, WorkDir: dir, Profile: profile, Up: true, Kind: project.Service, State: run.Running, Hold: run.HoldOwn}
}
