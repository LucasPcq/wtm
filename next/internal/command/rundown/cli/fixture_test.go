package cli

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/configtest"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/command/rundown"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
	"github.com/LucasPcq/wtm/next/internal/service/runjobs"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
)

const (
	mainPath    = "/repo/main"
	featA       = "feat/a"
	featB       = "feat/b"
	fingerprint = "0123456789abcdef01234567"
)

var startedAt = time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)

// runConfig is run.toml: two services, a stack, a shared service, dev the default profile.
var runConfig = project.Run{
	Declared: true,
	Jobs: []project.Job{
		{Name: "api", Kind: project.Service, Cmd: "api"},
		{Name: "web", Kind: project.Service, Cmd: "web"},
		{Name: "cache", Kind: project.Service, Cmd: "docker compose up -d cache", Stop: "docker compose down"},
		{Name: "db", Kind: project.Service, Cmd: "db", Scope: project.Shared},
	},
	Profiles: []project.Profile{{Name: "dev", Jobs: []string{"api", "web"}, Default: true}, {Name: "full", Jobs: []string{"db", "cache", "api", "web"}}},
}

type world struct {
	git    *gittest.Fake
	config *configtest.Fake
	locks  *operationtest.Locks
	daemon *daemon
}

func pathOf(branch string) string {
	return "/repo/wt-" + worktree.PathSegment(branch)
}

// newWorld runs full in feat/a (its db held with feat/b, its cache a stack) and dev in feat/b, whose stack's stop fails.
func newWorld() world {
	own := func(name, branch, profile string, hold run.Hold) run.Job {
		job := run.Job{Name: name, WorkDir: pathOf(branch), Profile: profile, Up: true, Kind: project.Service, State: run.Running, Hold: hold, Repo: mainPath}
		if hold == run.HoldDetached {
			job.StopCommand = "docker compose down"
		}
		return job
	}
	claim := func(branch string) run.Job {
		return run.Job{Name: "db", WorkDir: pathOf(branch), Profile: "full", Up: true, Kind: project.Service, Hold: run.HoldClaim, SharedDir: mainPath}
	}
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
		locks:  &operationtest.Locks{},
		daemon: &daemon{listed: []run.Job{
			{Name: "db", WorkDir: mainPath, Up: true, Kind: project.Service, State: run.Running, Hold: run.HoldOwn, SharedDir: mainPath},
			own("api", featA, "full", run.HoldOwn),
			own("cache", featA, "full", run.HoldDetached),
			claim(featA),
			own("web", featA, "full", run.HoldOwn),
			own("api", featB, "dev", run.HoldOwn),
			own("cache", featB, "dev", run.HoldDetached),
			claim(featB),
		}},
	}
}

func (w world) env() uicli.Env {
	return uicli.Env{Cwd: pathOf(featA), ProjectDir: mainPath}
}

func (w world) open(context.Context, string) (rundown.UseCase, error) {
	return rundown.UseCase{
		Stops:      runjobs.Stops{Daemon: w.daemon, Gone: repository.GoneIn(w.git), Located: repository.LocatedIn(w.git), Locate: repository.LocateIn(w.git)},
		Repository: repository.Service{Config: w.config, Git: w.git, MainPath: mainPath, Init: &domain.FollowUp{Command: "init"}, Restart: &domain.FollowUp{Command: "run daemon restart"}},
		Operations: operation.Service{
			Locker:    w.locks,
			Publisher: w.locks,
			NewOpID:   func() string { return "op-1" },
			Now:       func() time.Time { return startedAt },
		},
	}, nil
}

// A daemon stops every job it lists, but feat/b's cache, whose stop command fails, and releases every hold on db, as when main keeps it.
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

func (d *daemon) Stop(_ context.Context, req run.Stop) (run.Outcome, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if req.Name == "cache" && req.WorkDir == pathOf(featB) {
		return "", &run.Error{Code: run.CodeStopFailed, Message: "the stop command of cache failed (exit 4)"}
	}
	d.listed = slices.DeleteFunc(d.listed, func(job run.Job) bool { return job.WorkDir == req.WorkDir && job.Name == req.Name })
	// A shared service runs on while another hold of it is left.
	if req.Name == "db" && slices.ContainsFunc(d.listed, func(job run.Job) bool { return job.Name == "db" && (job.Hold == run.HoldClaim || job.MainHolds) }) {
		return run.OutcomeReleased, nil
	}
	return run.OutcomeStopped, nil
}
