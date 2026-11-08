package cli

import (
	"context"
	"time"

	"github.com/LucasPcq/wtm/next/internal/command/runps"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
)

const (
	mainPath  = "/acme"
	billing   = "/acme.trees/feat-billing"
	login     = "/acme.trees/fix-login"
	otherPath = "/other"
)

var clock = time.Date(2026, 11, 8, 14, 0, 0, 0, time.UTC)

// A world is the machine run ps reads: acme, whose feat/billing runs dev (worker crashed, migrate exited, api never started) and fix/login runs full (web and a stack running mailpit), both holding the db main runs and holds; Other adds a second repository running one job; JobsErr is the daemon's failure.
type world struct {
	Other   bool
	JobsErr error
	Nothing bool
}

func (w world) env() uicli.Env {
	return uicli.Env{Cwd: mainPath, ProjectDir: mainPath}
}

func (w world) open(context.Context, string) (runps.UseCase, error) {
	acme := runps.Project{StateDir: "/state", Records: map[string]worktree.Meta{"feat/billing": {Ordinal: 2}, "fix/login": {Ordinal: 3}}}
	acme.MainPath = mainPath
	acme.Run = project.Run{
		Jobs: []project.Job{
			{Name: "migrate", Kind: project.Task, Cmd: "pnpm migrate", Cwd: "packages/db"},
			{Name: "db", Kind: project.Service, Cmd: "postgres", Scope: project.Shared, Ports: map[string]int{"DB_PORT": 5432}},
			{Name: "web", Kind: project.Service, Cmd: "pnpm dev", Cwd: "apps/web", Ports: map[string]int{"WEB_PORT": 3000}, URL: &project.URL{Port: "WEB_PORT"}},
			{Name: "api", Kind: project.Service, Cmd: "pnpm dev", Cwd: "apps/api", Ports: map[string]int{"API_PORT": 4000}, URL: &project.URL{Port: "API_PORT"}},
			{Name: "worker", Kind: project.Service, Cmd: "pnpm start", Cwd: "apps/worker"},
			{Name: "mailpit", Kind: project.Service, Cmd: "true", Ports: map[string]int{"MAIL_PORT": 8025}},
			{Name: "stack", Kind: project.Service, Cmd: "docker compose up -d", Stop: "docker compose down", Runs: []string{"mailpit"}},
		},
		Profiles: []project.Profile{{Name: "dev", Jobs: []string{"migrate", "db", "web", "api", "worker"}}, {Name: "full", Jobs: []string{"web", "stack"}}},
	}
	acme.Worktrees = []worktree.Worktree{
		{Path: mainPath, Main: true, State: worktree.OnBranch{Branch: "main"}},
		{Path: login, State: worktree.OnBranch{Branch: "fix/login"}},
		{Path: billing, State: worktree.OnBranch{Branch: "feat/billing"}},
	}
	other := runps.Project{StateDir: "/other-state"}
	other.MainPath = otherPath
	other.Worktrees = []worktree.Worktree{{Path: otherPath, Main: true, State: worktree.OnBranch{Branch: "main"}}}

	at := func(d time.Duration) time.Time { return clock.Add(-d) }
	exit1, exit0 := 1, 0
	jobs := []run.Job{
		{Name: "db", WorkDir: mainPath, Repo: mainPath, SharedDir: mainPath, MainHolds: true, Namespace: "acme_main", Up: true, Kind: project.Service, State: run.Running, Hold: run.HoldOwn, Ports: map[string]int{"DB_PORT": 5432}, StartedAt: at(2 * time.Hour)},
		{Name: "db", WorkDir: billing, Repo: mainPath, SharedDir: mainPath, Namespace: "acme_feat_billing", Up: true, Hold: run.HoldClaim, StartedAt: at(12 * time.Minute)},
		{Name: "db", WorkDir: login, Repo: mainPath, SharedDir: mainPath, Namespace: "acme_fix_login", Up: true, Hold: run.HoldClaim, StartedAt: at(time.Hour)},
		{Name: "migrate", WorkDir: billing, Repo: mainPath, Profile: "dev", Kind: project.Task, State: run.Exited, Hold: run.HoldOwn, ExitCode: &exit0, StartedAt: at(12 * time.Minute), EndedAt: at(11 * time.Minute), LastLine: &run.Line{Text: "All migrations have been applied"}},
		{Name: "web", WorkDir: billing, Repo: mainPath, Profile: "dev", Up: true, Kind: project.Service, State: run.Running, Hold: run.HoldOwn, URL: "http://web.feat-billing.acme.localhost", Ports: map[string]int{"WEB_PORT": 3020}, StartedAt: at(12 * time.Minute), LastLine: &run.Line{Text: "GET /invoices 200 in 702ms"}},
		{Name: "worker", WorkDir: billing, Repo: mainPath, Profile: "dev", Kind: project.Service, State: run.Crashed, Hold: run.HoldOwn, ExitCode: &exit1, StartedAt: at(12 * time.Minute), EndedAt: at(2 * time.Minute), LastLine: &run.Line{Text: "Error: Cannot find module './jobs/send-invoice'"}},
		{Name: "web", WorkDir: login, Repo: mainPath, Profile: "full", Up: true, Kind: project.Service, State: run.Running, Hold: run.HoldOwn, Ports: map[string]int{"WEB_PORT": 3030}, StartedAt: at(time.Hour)},
		{Name: "stack", WorkDir: login, Repo: mainPath, Profile: "full", Up: true, Kind: project.Service, State: run.Running, Hold: run.HoldDetached, StartedAt: at(time.Hour), LastLine: &run.Line{Text: "Container mailpit-1  Started"}},
	}
	projects := []runps.Project{acme}
	if w.Other {
		jobs = append(jobs, run.Job{Name: "tool", WorkDir: otherPath, Repo: otherPath, Up: true, Kind: project.Service, State: run.Running, Hold: run.HoldOwn, StartedAt: at(3 * time.Minute)})
		projects = append(projects, other)
	}
	if w.Nothing || w.JobsErr != nil {
		jobs = nil
	}

	return runps.UseCase{
		Jobs: func(context.Context) ([]run.Job, error) { return jobs, w.JobsErr },
		Open: func(_ context.Context, dir string) (runps.Project, error) {
			for _, p := range projects {
				if p.MainPath == dir {
					return p, nil
				}
			}
			return runps.Project{}, &domain.Error{Kind: domain.KindPrecondition, Code: domain.CodeNotARepository, Message: dir + " is not a git repository"}
		},
		ReadLog: func(run.LogFile, int) ([]run.Line, error) { return nil, nil },
		Restart: &domain.FollowUp{Command: "daemon-restart"},
	}, nil
}
