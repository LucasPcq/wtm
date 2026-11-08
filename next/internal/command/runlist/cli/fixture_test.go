package cli

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/adapter/configtest"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/storetest"
	"github.com/LucasPcq/wtm/next/internal/command/runlist"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
)

const (
	mainPath = "/repo/main"
	featA    = "/repo/.trees/feat-a"
)

// A world is a repository whose run.toml declares web, api, a migrate task, a shared db and a stack running cache, in profiles dev (the default) and full; feat/a, numbered 1, runs dev, its api crashed, and holds db; migrate ran there once. Bare has no run.toml.
type world struct {
	Bare bool
}

func (w world) env() uicli.Env {
	return uicli.Env{Cwd: featA, ProjectDir: mainPath}
}

func (w world) open(context.Context, string) (runlist.UseCase, error) {
	declared := project.Run{
		Declared: true,
		Jobs: []project.Job{
			{Name: "web", Kind: project.Service, Cmd: "pnpm dev", Cwd: "apps/web", Ports: map[string]int{"PORT": 3000}, URL: &project.URL{Port: "PORT"}},
			{Name: "api", Kind: project.Service, Cmd: "pnpm dev", Cwd: "apps/api", Ports: map[string]int{"API_PORT": 4000}},
			{Name: "migrate", Kind: project.Task, Cmd: "pnpm migrate"},
			{Name: "db", Kind: project.Service, Cmd: "postgres", Scope: project.Shared, Ports: map[string]int{"DB_PORT": 5432}},
			{Name: "cache", Kind: project.Service, Cmd: "true", Ports: map[string]int{"CACHE_PORT": 6379}},
			{Name: "stack", Kind: project.Service, Cmd: "docker compose up -d", Stop: "docker compose down", Runs: []string{"cache"}},
		},
		Profiles: []project.Profile{
			{Name: "dev", Jobs: []string{"db", "api", "web"}, Default: true},
			{Name: "full", Jobs: []string{"migrate", "db", "stack", "api", "web"}},
		},
	}
	if w.Bare {
		declared = project.Run{}
	}
	exit1 := 1
	jobs := []run.Job{
		{Name: "web", WorkDir: featA, Profile: "dev", Up: true, State: run.Running, Hold: run.HoldOwn, URL: "http://web.feat-a.acme.localhost"},
		{Name: "api", WorkDir: featA, Profile: "dev", State: run.Crashed, Hold: run.HoldOwn, ExitCode: &exit1},
		{Name: "db", WorkDir: featA, SharedDir: mainPath, Up: true, Hold: run.HoldClaim},
	}

	git := gittest.NewFake(gittest.FakeParams{Main: mainPath, Branch: "main", Worktrees: []worktree.Worktree{{Path: featA, State: worktree.OnBranch{Branch: "feat/a"}}}})
	store := storetest.NewFake(storetest.FakeParams{Meta: map[string]worktree.Meta{"feat/a": {Ordinal: 1}}})
	config := project.Config{Worktrees: project.Worktrees{BasePath: "../.trees", BaseBranch: "main"}}
	return runlist.UseCase{
		Git:  git,
		Meta: store,
		Jobs: func(context.Context) ([]run.Job, error) { return jobs, nil },
		ReadLog: func(file run.LogFile, _ int) ([]run.Line, error) {
			if file.Job == "migrate" {
				return []run.Line{{Text: "migrated"}}, nil
			}
			return nil, nil
		},
		StateDir:   "/state",
		Repository: repository.Service{Config: configtest.NewFake(configtest.FakeParams{Config: &config, Run: declared}), Git: git, MainPath: mainPath},
	}, nil
}
