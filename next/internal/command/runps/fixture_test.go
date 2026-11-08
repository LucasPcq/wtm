package runps_test

import (
	"context"
	"time"

	"github.com/LucasPcq/wtm/next/internal/command/runps"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

const (
	mainPath    = "/acme"
	billingPath = "/acme.trees/feat-billing"
	loginPath   = "/acme.trees/fix-login"
	otherPath   = "/other"
)

var (
	started = time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	ended   = started.Add(10 * time.Minute)
)

// acme is a repository whose run.toml declares web, api, worker, a migrate task, a stack running mailpit, and a shared db; profile dev runs migrate, db, web, api and worker, profile full web and the stack.
var acmeRun = project.Run{
	Jobs: []project.Job{
		{Name: "migrate", Kind: project.Task, Cmd: "pnpm migrate", Cwd: "packages/db"},
		{Name: "db", Kind: project.Service, Cmd: "postgres", Scope: project.Shared, Ports: map[string]int{"DB_PORT": 5432}},
		{Name: "web", Kind: project.Service, Cmd: "pnpm dev", Cwd: "apps/web", Ports: map[string]int{"WEB_PORT": 3000}, URL: &project.URL{Port: "WEB_PORT"}},
		{Name: "api", Kind: project.Service, Cmd: "pnpm dev", Cwd: "apps/api", Ports: map[string]int{"API_PORT": 4000}, URL: &project.URL{Port: "API_PORT"}},
		{Name: "worker", Kind: project.Service, Cmd: "pnpm start", Cwd: "apps/worker"},
		{Name: "mailpit", Kind: project.Service, Cmd: "true", Ports: map[string]int{"MAIL_PORT": 8025}},
		{Name: "stack", Kind: project.Service, Cmd: "docker compose up -d", Stop: "docker compose down", Runs: []string{"mailpit"}},
	},
	Profiles: []project.Profile{
		{Name: "dev", Jobs: []string{"migrate", "db", "web", "api", "worker"}, Default: true},
		{Name: "full", Jobs: []string{"web", "stack", "mailpit"}},
	},
}

func acme() runps.Project {
	p := runps.Project{StateDir: "/state", Records: map[string]worktree.Meta{"feat/billing": {Ordinal: 2}, "fix/login": {Ordinal: 3}}}
	p.MainPath, p.Run = mainPath, acmeRun
	p.Worktrees = []worktree.Worktree{
		{Path: mainPath, Main: true, State: worktree.OnBranch{Branch: "main"}},
		{Path: loginPath, State: worktree.OnBranch{Branch: "fix/login"}},
		{Path: billingPath, State: worktree.OnBranch{Branch: "feat/billing"}},
	}
	return p
}

func other() runps.Project {
	p := runps.Project{StateDir: "/other-state"}
	p.MainPath = otherPath
	p.Worktrees = []worktree.Worktree{{Path: otherPath, Main: true, State: worktree.OnBranch{Branch: "main"}}}
	return p
}

// jobs are the daemon's: feat/billing runs dev (worker crashed, migrate exited, db claimed), fix/login runs full (web, its stack) and claims db, main runs db for them and holds it, and another repository runs one job.
func jobs() []run.Job {
	exit1, exit0 := 1, 0
	return []run.Job{
		{Name: "db", WorkDir: mainPath, Repo: mainPath, SharedDir: mainPath, MainHolds: true, Namespace: "acme_main", Up: true, Kind: project.Service, State: run.Running, Hold: run.HoldOwn, Ports: map[string]int{"DB_PORT": 5432}, StartedAt: started},
		{Name: "db", WorkDir: billingPath, Repo: mainPath, SharedDir: mainPath, Namespace: "acme_feat_billing", Up: true, Hold: run.HoldClaim, StartedAt: started.Add(time.Minute)},
		{Name: "db", WorkDir: loginPath, Repo: mainPath, SharedDir: mainPath, Namespace: "acme_fix_login", Up: true, Hold: run.HoldClaim, StartedAt: started.Add(2 * time.Minute)},
		{Name: "migrate", WorkDir: billingPath, Repo: mainPath, Profile: "dev", Kind: project.Task, State: run.Exited, Hold: run.HoldOwn, ExitCode: &exit0, StartedAt: started, EndedAt: started.Add(4 * time.Second)},
		{Name: "web", WorkDir: billingPath, Repo: mainPath, Profile: "dev", Up: true, Kind: project.Service, State: run.Running, Hold: run.HoldOwn, URL: "http://web.feat-billing.acme.localhost", Ports: map[string]int{"WEB_PORT": 3020}, StartedAt: started, LastLine: &run.Line{At: ended, Text: "GET / 200"}},
		{Name: "worker", WorkDir: billingPath, Repo: mainPath, Profile: "dev", Kind: project.Service, State: run.Crashed, Hold: run.HoldOwn, ExitCode: &exit1, StartedAt: started, EndedAt: ended, LastLine: &run.Line{At: ended, Text: "Error: Cannot find module"}},
		{Name: "stack", WorkDir: loginPath, Repo: mainPath, Profile: "full", Up: true, Kind: project.Service, State: run.Running, Hold: run.HoldDetached, StartedAt: started},
		{Name: "web", WorkDir: loginPath, Repo: mainPath, Profile: "full", Up: true, Kind: project.Service, State: run.Running, Hold: run.HoldOwn, Ports: map[string]int{"WEB_PORT": 3030}, StartedAt: started},
		{Name: "tool", WorkDir: otherPath, Repo: otherPath, Up: true, Kind: project.Service, State: run.Running, Hold: run.HoldOwn, StartedAt: started},
	}
}

// A seed is the machine run ps reads: the daemon's jobs and its error, the repositories by any dir in them (an absent one does not read), and the jobs whose log holds a line, by their file.
type seed struct {
	Jobs     []run.Job
	JobsErr  error
	Projects []runps.Project
	Logged   map[run.LogFile]run.Line
}

func usecase(s seed) runps.UseCase {
	return runps.UseCase{
		Jobs: func(context.Context) ([]run.Job, error) { return s.Jobs, s.JobsErr },
		Open: func(_ context.Context, dir string) (runps.Project, error) {
			for _, p := range s.Projects {
				for _, w := range p.Worktrees {
					if w.Path == dir {
						return p, nil
					}
				}
			}
			return runps.Project{}, &domain.Error{Kind: domain.KindPrecondition, Code: domain.CodeNotARepository, Message: dir + " is not a git repository"}
		},
		ReadLog: func(file run.LogFile, _ int) ([]run.Line, error) {
			if line, ok := s.Logged[file]; ok {
				return []run.Line{line}, nil
			}
			return nil, nil
		},
		Restart: &domain.FollowUp{Command: "daemon-restart"},
	}
}
