package runlist_test

import (
	"context"
	"slices"

	"github.com/LucasPcq/wtm/next/internal/adapter/configtest"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/storetest"
	"github.com/LucasPcq/wtm/next/internal/command/runlist"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

const (
	mainPath = "/repo/main"
	featA    = "/repo/.trees/feat-a"
)

// declared is a run.toml with web and api, a migrate task, a shared db and a stack running cache; dev, the default profile, runs api and web, full every job but the stack's own cache.
var declared = project.Run{
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
		{Name: "dev", Jobs: []string{"api", "web"}, Default: true},
		{Name: "full", Jobs: []string{"migrate", "db", "stack", "api", "web"}},
	},
}

// A seed is a repository to list: its run.toml and the error loading it, the daemon's jobs and its error, and the jobs whose log holds a line, in every worktree; feat/a is numbered 1.
type seed struct {
	Run     project.Run
	RunErr  error
	Jobs    []run.Job
	JobsErr error
	Logged  []string
	Unread  bool // feat/a's record does not read
}

func usecase(s seed) runlist.UseCase {
	git := gittest.NewFake(gittest.FakeParams{Main: mainPath, Branch: "main", Worktrees: []worktree.Worktree{{Path: featA, State: worktree.OnBranch{Branch: "feat/a"}}}})
	var unread []string
	if s.Unread {
		unread = []string{"feat/a"}
	}
	store := storetest.NewFake(storetest.FakeParams{Meta: map[string]worktree.Meta{"feat/a": {Ordinal: 1}}, Unreadable: unread})
	config := project.Config{Worktrees: project.Worktrees{BasePath: "../.trees", BaseBranch: "main"}}
	return runlist.UseCase{
		Git:  git,
		Meta: store,
		Jobs: func(context.Context) ([]run.Job, error) { return s.Jobs, s.JobsErr },
		ReadLog: func(file run.LogFile, _ int) ([]run.Line, error) {
			if slices.Contains(s.Logged, file.Job) {
				return []run.Line{{Text: "ran"}}, nil
			}
			return nil, nil
		},
		StateDir:   "/state",
		Repository: repository.Service{Config: configtest.NewFake(configtest.FakeParams{Config: &config, Run: s.Run, Fail: map[string]error{"LoadRun": s.RunErr}}), Git: git, MainPath: mainPath},
	}
}
