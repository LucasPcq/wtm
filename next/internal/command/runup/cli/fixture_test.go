package cli

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/configtest"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/storetest"
	"github.com/LucasPcq/wtm/next/internal/command/runup"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
)

const (
	mainPath    = "/repo/main"
	featA       = "feat/a"
	featB       = "feat/b"
	fingerprint = "0123456789abcdef01234567"
)

var startedAt = time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)

// runConfig is run.toml: a task, then two services, dev the default profile; the task fails in feat/b.
var runConfig = project.Run{
	Declared: true,
	Jobs: []project.Job{
		{Name: "migrate", Kind: project.Task, Cmd: "migrate"},
		{Name: "api", Kind: project.Service, Cmd: "api", Ports: map[string]int{"PORT": 4000}, URL: &project.URL{Port: "PORT"}},
		{Name: "web", Kind: project.Service, Cmd: "web", Ports: map[string]int{"PORT": 3000}, URL: &project.URL{Port: "PORT"}},
	},
	Profiles: []project.Profile{{Name: "dev", Jobs: []string{"migrate", "api", "web"}, Default: true}},
}

type world struct {
	git    *gittest.Fake
	store  *storetest.Fake
	config *configtest.Fake
	locks  *operationtest.Locks
	daemon *daemon
}

func pathOf(branch string) string {
	return "/repo/wt-" + worktree.PathSegment(branch)
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
		store:  storetest.NewFake(storetest.FakeParams{Meta: map[string]worktree.Meta{featA: {SourceBranch: "main", Ordinal: 1}, featB: {SourceBranch: "main", Ordinal: 2}}}),
		config: configtest.NewFake(configtest.FakeParams{Config: &project.Config{Worktrees: project.Worktrees{BasePath: project.DefaultBasePath, BaseBranch: project.DefaultBaseBranch}}, Run: runConfig}),
		locks:  &operationtest.Locks{},
		daemon: &daemon{},
	}
}

func (w world) env() uicli.Env {
	return uicli.Env{Cwd: pathOf(featA), ProjectDir: mainPath}
}

func (w world) open(context.Context, string) (runup.UseCase, error) {
	return runup.UseCase{
		Git:        w.git,
		Meta:       w.store,
		EnvFiles:   w.store,
		Daemon:     w.daemon,
		Listening:  answering,
		StateDir:   "/repo/main/.git/wtm",
		CommonDir:  "/repo/main/.git",
		Repository: repository.Service{Config: w.config, Git: w.git, MainPath: mainPath, Init: &domain.FollowUp{Command: "init"}, Restart: &domain.FollowUp{Command: "run daemon restart"}},
		Operations: operation.Service{
			Locker:    w.locks,
			Publisher: w.locks,
			NewOpID:   func() string { return "op-1" },
			Now:       func() time.Time { return startedAt },
		},
		Provision: provision.Service{Git: w.git, Meta: w.store},
	}, nil
}

// A daemon starts every job, but the task of feat/b, which exits 3 after printing its error, and lists what it started.
type daemon struct {
	mu     sync.Mutex
	listed []run.Job
}

func (d *daemon) Jobs(context.Context) ([]run.Job, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Clone(d.listed), nil
}

func (d *daemon) Ensure(context.Context) error { return nil }

func (d *daemon) Start(_ context.Context, spec run.Spec, out func(run.Line)) (run.Job, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if spec.Job.Name == "migrate" && spec.WorkDir == pathOf(featB) {
		out(run.Line{At: startedAt, Text: "error: relation users does not exist"})
		exit := 3
		return run.Job{}, &run.Error{Code: run.CodeTaskFailed, Message: "run daemon: task migrate exited with code 3", ExitCode: &exit}
	}
	job := run.Job{Name: spec.Job.Name, WorkDir: spec.WorkDir, Up: spec.Job.Kind == project.Service, Kind: spec.Job.Kind, State: run.Running, URL: spec.URL(11080)}
	d.listed = append(d.listed, job)
	return job, nil
}

func (d *daemon) StopAll(context.Context, run.StopAll) ([]run.StoppedJob, error) { return nil, nil }

// answering is a loopback where something answers on every port.
func answering(_ context.Context, ports []int, _ time.Duration) map[int]bool {
	answered := map[int]bool{}
	for _, port := range ports {
		answered[port] = true
	}
	return answered
}
