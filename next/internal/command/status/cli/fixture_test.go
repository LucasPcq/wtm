package cli

import (
	"cmp"
	"context"
	"errors"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/storetest"
	"github.com/LucasPcq/wtm/next/internal/command/status"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
)

const (
	mainPath  = "/repo/main"
	createdAt = "2026-10-16T12:00:00Z"
)

// clock is the time a test draws ages against: an hour after the jobs started.
var clock = time.Date(2026, 10, 16, 14, 0, 0, 0, time.UTC)

// configured is a project's config and run.toml as repository.Service loads them.
type configured struct {
	err    error
	run    project.Run
	runErr error
}

func (c configured) Load() (project.Config, project.Global, error) {
	return project.Config{
		Worktrees: project.Worktrees{BasePath: "../.trees", BaseBranch: "main"},
		Env:       project.Env{Strategy: project.EnvMain, Files: []project.EnvFile{{Target: ".env"}, {Target: "web/.env"}}},
	}, project.Global{}, c.err
}

func (c configured) LoadRun() (project.Run, error) {
	return c.run, c.runErr
}

// A world is a repository of every kind of worktree status draws: feat/a ahead of main and origin with uncommitted work, its web job up and its api crashed, missing a .env; feat/old, created before the isolation choice; a detached one; Cwd is where the command runs.
type world struct {
	Cwd       string
	ConfigErr error
	NoRun     bool
	RunErr    error
	JobsErr   error
	PRsErr    error
	Merged    bool // feat/a's pull request is merged
	Gone      bool // feat/old's directory is gone
}

func (w world) env() uicli.Env {
	cwd := cmp.Or(w.Cwd, mainPath)
	return uicli.Env{Cwd: cwd, ProjectDir: cwd}
}

func (w world) open(context.Context, string) (status.UseCase, error) {
	git := gittest.NewFake(gittest.FakeParams{
		Main: mainPath, Branch: "main", Remote: []string{"main", "feat/a"},
		Worktrees: []worktree.Worktree{
			{Path: "/repo/.trees/feat-a", State: worktree.OnBranch{Branch: "feat/a"}},
			{Path: "/repo/.trees/feat-old", State: worktree.OnBranch{Branch: "feat/old"}},
			{Path: "/repo/.trees/detached", State: worktree.Detached{}},
		},
		Commits: map[worktree.Ref][]string{
			worktree.LocalBranch{Name: "feat/a"}: {"Add the login form", "Wire the session"},
			worktree.LocalBranch{Name: "main"}:   {"Bump"},
		},
		Changes:   map[string][]worktree.Change{"/repo/.trees/feat-a": {{Path: "a.go", Status: worktree.ChangeModified}, {Path: "b.go", Status: worktree.ChangeUntracked}}},
		Diffstats: map[string]worktree.Diffstat{"/repo/.trees/feat-a": {Staged: 1, Unstaged: 1, Insertions: 12, Deletions: 3}},
		Gone:      gone(w.Gone),
	})
	store := storetest.NewFake(storetest.FakeParams{
		Meta: map[string]worktree.Meta{
			"feat/a":   {CreatedAt: createdAt, SourceBranch: "main", Ordinal: 1, Isolation: project.Isolated, EnvStrategy: project.EnvMain},
			"feat/old": {CreatedAt: createdAt, SourceBranch: "main", EnvStrategy: project.EnvMain},
		},
		Files: map[string]string{
			mainPath + "/.env":               "A=1\n",
			mainPath + "/web/.env":           "PORT=3000\n",
			"/repo/.trees/feat-a/.env":       "A=1\n",
			"/repo/.trees/feat-old/.env":     "A=1\n",
			"/repo/.trees/feat-old/web/.env": "PORT=3000\n",
			"/repo/.trees/detached/.env":     "A=1\n",
			"/repo/.trees/detached/web/.env": "PORT=3000\n",
		},
	})
	started, exit := clock.Add(-time.Hour), 1
	runConfig := project.Run{Declared: true, Jobs: []project.Job{
		{Name: "web", Kind: project.Service, Cmd: "pnpm dev", Ports: map[string]int{"PORT": 3000}, URL: &project.URL{Port: "PORT"}},
		{Name: "api", Kind: project.Service, Cmd: "pnpm api"},
	}}
	if w.NoRun {
		runConfig = project.Run{}
	}

	return status.UseCase{
		Git:      git,
		Meta:     store,
		EnvFiles: store,
		Jobs: func(context.Context) ([]run.Job, error) {
			return []run.Job{
				{Name: "web", Kind: project.Service, WorkDir: "/repo/.trees/feat-a", Up: true, State: run.Running, URL: "http://web.feat-a.main.localhost:11080", StartedAt: started},
				{Name: "api", Kind: project.Service, WorkDir: "/repo/.trees/feat-a", State: run.Crashed, ExitCode: &exit},
			}, w.JobsErr
		},
		PullRequests: func(context.Context, []string) ([]worktree.PullRequest, error) {
			pr := worktree.PullRequest{Number: 12, URL: "https://github.com/acme/acme/pull/12", Branch: "feat/a", Head: "Wire the session", State: worktree.PullRequestOpen, Title: "Add the login form", Draft: true, Checks: worktree.Checks{Passed: 3, Pending: 1}, Decision: "REVIEW_REQUIRED"}
			if w.Merged {
				pr.State = worktree.PullRequestMerged
			}
			return []worktree.PullRequest{pr}, w.PRsErr
		},
		Repository: repository.Service{Config: configured{err: w.ConfigErr, run: runConfig, runErr: w.RunErr}, Git: git, MainPath: mainPath, Init: &domain.FollowUp{Command: domain.CommandInit}},
	}, nil
}

// enters is a terminal on which the user keeps pressing enter.
type enters struct{}

func (enters) Read(p []byte) (int, error) {
	time.Sleep(time.Millisecond)
	p[0] = '\r'
	return 1, nil
}

// errInvalidRun is run.toml that does not read.
var errInvalidRun = errors.New(`invalid run config: unknown addressing "nope"`)

// gone is feat/old's directory, when it is gone.
func gone(on bool) []string {
	if !on {
		return nil
	}
	return []string{"/repo/.trees/feat-old"}
}
