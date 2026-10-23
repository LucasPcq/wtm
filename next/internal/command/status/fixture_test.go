package status_test

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/storetest"
	"github.com/LucasPcq/wtm/next/internal/command/status"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

const (
	mainPath  = "/repo/main"
	createdAt = "2026-10-16T12:00:00Z"
)

// configured is a project's config and run.toml as repository.Service loads them, each with the error loading it.
type configured struct {
	config project.Config
	err    error
	run    project.Run
	runErr error
}

func (c configured) Load() (project.Config, project.Global, error) {
	return c.config, project.Global{}, c.err
}

func (c configured) LoadRun() (project.Run, error) {
	return c.run, c.runErr
}

// A seed is a repository to read: its git, its meta records (and those that do not read), its .env files by path, its declared .env files and env strategy, its run.toml, the daemon's jobs and GitHub's pull requests, each with the error reading it.
type seed struct {
	Git        gittest.FakeParams
	Meta       map[string]worktree.Meta
	Unreadable []string
	Files      map[string]string
	EnvFiles   []project.EnvFile
	Strategy   project.EnvStrategy
	Run        project.Run
	RunErr     error
	ConfigErr  error
	Jobs       []run.Job
	JobsErr    error
	PRs        []worktree.PullRequest
	PRsErr     error
}

// A world is status's use case over the fakes a seed describes, with the branches GitHub was asked of, once per ask.
type world struct {
	usecase status.UseCase
	git     *gittest.Fake
	store   *storetest.Fake
	asked   *[][]string
}

func newWorld(s seed) world {
	if s.Git.Main == "" {
		s.Git.Main, s.Git.Branch = mainPath, "main"
	}

	git := gittest.NewFake(s.Git)
	store := storetest.NewFake(storetest.FakeParams{Meta: s.Meta, Unreadable: s.Unreadable, Files: s.Files})
	var asked [][]string
	config := project.Config{Worktrees: project.Worktrees{BasePath: "../.trees", BaseBranch: "main"}, Env: project.Env{Strategy: s.Strategy, Files: s.EnvFiles}}
	return world{
		usecase: status.UseCase{
			Git:      git,
			Meta:     store,
			EnvFiles: store,
			Jobs:     func(context.Context) ([]run.Job, error) { return s.Jobs, s.JobsErr },
			PullRequests: func(_ context.Context, branches []string) ([]worktree.PullRequest, error) {
				asked = append(asked, branches)
				return s.PRs, s.PRsErr
			},
			Repository: repository.Service{Config: configured{config: config, err: s.ConfigErr, run: s.Run, runErr: s.RunErr}, Git: git, MainPath: mainPath},
		},
		git:   git,
		store: store,
		asked: &asked,
	}
}

// linked is a linked worktree under /repo/.trees on branch.
func linked(branch string) worktree.Worktree {
	return worktree.Worktree{Path: "/repo/.trees/" + branch, State: worktree.OnBranch{Branch: branch}}
}

// isolating is a run.toml that isolates a worktree: a web job publishing its port.
func isolating() project.Run {
	return project.Run{Declared: true, Jobs: []project.Job{
		{Name: "web", Kind: project.Service, Cmd: "pnpm dev", Ports: map[string]int{"PORT": 3000}, URL: &project.URL{Port: "PORT"}},
	}}
}

// failingChanges is git whose Changes fails with err for the worktree at path.
type failingChanges struct {
	status.Git
	path string
	err  error
}

func (f failingChanges) Changes(ctx context.Context, dir string) ([]worktree.Change, error) {
	if dir == f.path {
		return nil, f.err
	}
	return f.Git.Changes(ctx, dir)
}
