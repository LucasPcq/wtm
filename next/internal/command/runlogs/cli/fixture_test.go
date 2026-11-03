package cli

import (
	"context"
	"net"
	"slices"
	"sync"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/configtest"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/storetest"
	"github.com/LucasPcq/wtm/next/internal/command/runlogs"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
)

const (
	mainPath = "/repo/main"
	stateDir = "/repo/main/.git/wtm"
	featA    = "feat/a"
	featB    = "feat/b"
)

// altScreen is what a program writes as it takes the terminal's alternate screen.
const altScreen = "\x1b[?1049h"

var at = time.Date(2026, 11, 3, 20, 17, 21, 0, time.UTC)

// runConfig is run.toml: a task, two services, one never started.
var runConfig = project.Run{
	Declared: true,
	Jobs: []project.Job{
		{Name: "migrate", Kind: project.Task, Cmd: "migrate"},
		{Name: "api", Kind: project.Service, Cmd: "api"},
		{Name: "web", Kind: project.Service, Cmd: "web"},
		{Name: "never", Kind: project.Service, Cmd: "never"},
	},
}

// A world is feat/a, where migrate ran and api and web run, and feat/b, where nothing ran.
type world struct {
	git    *gittest.Fake
	config *configtest.Fake
	daemon *daemon
	logs   map[string][]run.Line // by log directory and job, joined by a space
}

func pathOf(branch string) string {
	return "/repo/wt-" + worktree.PathSegment(branch)
}

func logDirOf(branch string) string {
	return stateDir + "/logs/" + worktree.StateSegment(branch)
}

func newWorld() world {
	return world{
		git: gittest.NewFake(gittest.FakeParams{
			Main:   mainPath,
			Branch: "main",
			Worktrees: []worktree.Worktree{
				{Path: pathOf(featA), State: worktree.OnBranch{Branch: featA}},
				{Path: pathOf(featB), State: worktree.OnBranch{Branch: featB}},
			},
		}),
		config: configtest.NewFake(configtest.FakeParams{Config: &project.Config{Worktrees: project.Worktrees{BasePath: project.DefaultBasePath, BaseBranch: project.DefaultBaseBranch}}, Run: runConfig}),
		daemon: &daemon{
			listed: []run.Job{
				{Name: "api", WorkDir: pathOf(featA), Up: true, Kind: project.Service, State: run.Running, Hold: run.HoldOwn, URL: "http://api.feat-a.localhost"},
				{Name: "web", WorkDir: pathOf(featA), Up: true, Kind: project.Service, State: run.Running, Hold: run.HoldOwn},
			},
			past: map[string][]run.Line{"api": {{At: at, Text: "api ready"}}},
			live: map[string][]run.Line{"api": {{At: at.Add(time.Minute), Text: "GET /users 200"}}},
		},
		logs: map[string][]run.Line{
			logDirOf(featA) + " migrate": {{At: at, Text: "migrating"}, {At: at.Add(time.Second), Text: "migrated"}},
			logDirOf(featA) + " api":     {{At: at, Text: "api ready"}},
			logDirOf(featA) + " web":     {{At: at, Text: "web ready"}},
		},
	}
}

func (w world) env() uicli.Env {
	return uicli.Env{Cwd: pathOf(featA), ProjectDir: mainPath}
}

func (w world) open(context.Context, string) (runlogs.UseCase, error) {
	return runlogs.UseCase{
		Git:        w.git,
		Daemon:     w.daemon,
		ReadLog:    w.readLog,
		ScanLog:    w.scanLog,
		Meta:       storetest.NewFake(storetest.FakeParams{}),
		EnvFiles:   storetest.NewFake(storetest.FakeParams{}),
		StateDir:   stateDir,
		Repository: repository.Service{Config: w.config, Git: w.git, MainPath: mainPath, Init: &domain.FollowUp{Command: "init"}, Restart: &domain.FollowUp{Command: "run daemon restart"}},
	}, nil
}

func (w world) scanLog(file run.LogFile, visit func(run.Line) bool) error {
	for _, line := range w.logs[file.Dir+" "+file.Job] {
		if !visit(line) {
			break
		}
	}
	return nil
}

func (w world) readLog(file run.LogFile, n int) ([]run.Line, error) {
	lines := w.logs[file.Dir+" "+file.Job]
	return lines[max(0, len(lines)-n):], nil
}

// A daemon lists its jobs; a follow of one of them sends its last lines, then its live lines, then waits for the follow to end when wait is set, else ends with the job.
type daemon struct {
	mu     sync.Mutex
	listed []run.Job
	past   map[string][]run.Line // by job: the last lines it keeps
	live   map[string][]run.Line // by job: what it prints while followed
	wait   func()                // called once the live lines are sent; the follow then waits for its end
	refuse error                 // what a tail of a job's lines fails with: the legacy daemon's
	broke  error                 // what a tail fails with once it sent the job's last lines
	skewed *run.Skewed
}

func (d *daemon) Jobs(context.Context) ([]run.Job, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.skewed != nil {
		return slices.Clone(d.listed), d.skewed
	}
	return slices.Clone(d.listed), nil
}

func (d *daemon) Lines(ctx context.Context, req run.Tail, out func(run.Line)) (run.Job, error) {
	if d.refuse != nil {
		return run.Job{}, d.refuse
	}
	if d.broke != nil {
		for _, line := range d.past[req.Name] {
			out(line)
		}
		return run.Job{}, d.broke
	}
	if !req.Follow {
		return run.Job{}, &run.Error{Code: run.CodeJobNotFound}
	}
	for _, line := range append(slices.Clone(d.past[req.Name]), d.live[req.Name]...) {
		out(line)
	}
	if d.wait != nil {
		d.wait()
		<-ctx.Done()
		return run.Job{}, ctx.Err()
	}
	return run.Job{Name: req.Name, WorkDir: req.WorkDir}, nil
}

// Attach gives a terminal the job never prints to.
func (d *daemon) Attach(context.Context, run.Window) (net.Conn, error) {
	view, _ := net.Pipe()
	return view, nil
}

func (d *daemon) Resize(context.Context, run.Window) error { return nil }
