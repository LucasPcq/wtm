package runlogs_test

import (
	"context"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/config"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/joblog"
	"github.com/LucasPcq/wtm/next/internal/adapter/store"
	"github.com/LucasPcq/wtm/next/internal/command/runlogs"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

const (
	featA       = "feat/a"
	featB       = "feat/b"
	gone        = "feat/gone"
	projectToml = "[worktrees]\nbase_path = \"../.trees\"\nbase_branch = \"main\"\n"
	// runToml declares two services, a task, a service that crashes, a shared service, a job never started and a compose stack.
	runToml = `
[[job]]
name = "api"
kind = "service"
cmd = "api"

[[job]]
name = "web"
kind = "service"
cmd = "web"

[[job]]
name = "migrate"
kind = "task"
cmd = "migrate"

[[job]]
name = "crashy"
kind = "service"
cmd = "crashy"

[[job]]
name = "db"
kind = "service"
cmd = "db"
scope = "shared"

[[job]]
name = "never"
kind = "service"
cmd = "never"

[[job]]
name = "stack"
kind = "service"
cmd = "docker compose up -d"
stop = "docker compose down"
`
)

var at = time.Date(2026, 11, 3, 20, 17, 21, 0, time.UTC)

type world struct {
	main     string
	root     string
	stateDir string
	daemon   *daemon
	usecase  runlogs.UseCase
}

// newWorld is a repository with two worktrees and one whose directory is gone; run.toml declares runToml; nothing ran yet.
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
			{Path: linkedPath(root, gone), State: worktree.OnBranch{Branch: gone}},
		},
		Gone: []string{linkedPath(root, gone)},
	})
	stateDir := filepath.Join(main, ".git", "wtm")
	writeFile(t, filepath.Join(stateDir, "config.toml"), projectToml)
	writeFile(t, filepath.Join(stateDir, "run.toml"), runToml)
	w := &world{main: main, root: root, stateDir: stateDir, daemon: &daemon{lines: map[string][]script{}}}
	w.usecase = runlogs.UseCase{
		Git:        repo,
		Daemon:     w.daemon,
		ReadLog:    joblog.Tail,
		ScanLog:    joblog.Each,
		Meta:       store.Meta{StateDir: stateDir},
		EnvFiles:   store.EnvFiles{},
		StateDir:   stateDir,
		Repository: repository.Service{Config: config.Project{StateDir: stateDir, GlobalPath: filepath.Join(root, "no-global.toml")}, Git: repo, MainPath: main, Restart: &domain.FollowUp{Command: "run daemon restart"}},
	}
	return w
}

func linkedPath(root, name string) string {
	return filepath.Join(root, "wt-"+worktree.PathSegment(name))
}

// A script is one answer of the daemon to lines: the lines it sends, then how it ends.
type script struct {
	before func() // called as the follow starts
	lines  []run.Line
	wait   bool // it waits for the follow's context to end after its lines
	err    error
}

// A daemon is the run daemon a test scripts: the jobs it lists, whether it could not tell them or is of another build, and what each lines of a job answers, in turn (the last one again once they ran out); every tail it was asked.
type daemon struct {
	mu      sync.Mutex
	listed  []run.Job
	jobsErr error
	skewed  *run.Skewed
	lines   map[string][]script // by worktree path and job name, joined by a space
	asked   []run.Tail
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
	if d.skewed != nil {
		return slices.Clone(d.listed), d.skewed
	}
	return slices.Clone(d.listed), nil
}

func (d *daemon) Lines(ctx context.Context, req run.Tail, out func(run.Line)) (run.Job, error) {
	d.mu.Lock()
	d.asked = append(d.asked, req)
	scripts := d.lines[key(req.WorkDir, req.Name)]
	answer := script{err: &run.Error{Code: run.CodeJobNotFound, Message: "no such job"}}
	if len(scripts) > 0 {
		answer = scripts[0]
		if len(scripts) > 1 {
			d.lines[key(req.WorkDir, req.Name)] = scripts[1:]
		}
	}
	d.mu.Unlock()

	if answer.before != nil {
		answer.before()
	}
	for _, line := range answer.lines {
		out(line)
	}
	if answer.wait {
		<-ctx.Done()
		return run.Job{}, ctx.Err()
	}
	return run.Job{Name: req.Name, WorkDir: req.WorkDir}, answer.err
}

// ran is job of kind as the daemon lists it in dir.
func ran(name, dir string, state run.State) run.Job {
	return run.Job{Name: name, WorkDir: dir, Up: state == run.Running, Kind: project.Service, State: state, Hold: run.HoldOwn, URL: "http://" + name + ".localhost"}
}
