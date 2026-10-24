package cli

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/adapter/configtest"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/storetest"
	"github.com/LucasPcq/wtm/next/internal/command/list"
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

// A world is a repository of every kind of worktree list draws: dirty with commits ahead of main and of origin and a job up, locked, detached, gone, with a pull request, and a commit on origin's main not fetched yet; Cwd is where the command runs.
type world struct {
	Cwd       string
	ConfigErr error
	PRsErr    error
	JobsErr   error
	Fail      map[string]error // git's failures, by method
}

func (w world) env() uicli.Env {
	cwd := w.Cwd
	if cwd == "" {
		cwd = mainPath
	}
	return uicli.Env{Cwd: cwd, ProjectDir: cwd}
}

func (w world) open(context.Context, string) (list.UseCase, error) {
	git := gittest.NewFake(gittest.FakeParams{
		Main: mainPath, Branch: "main", Remote: []string{"main", "feat/a"},
		Worktrees: []worktree.Worktree{
			{Path: "/repo/.trees/feat-a", State: worktree.OnBranch{Branch: "feat/a"}},
			{Path: "/repo/.trees/feat-b", State: worktree.OnBranch{Branch: "feat/b"}, Locked: true},
			{Path: "/repo/.trees/detached", State: worktree.Detached{}},
			{Path: "/repo/.trees/gone", State: worktree.OnBranch{Branch: "feat/gone"}},
		},
		Gone:    []string{"/repo/.trees/gone"},
		Dirty:   []string{"/repo/.trees/feat-a"},
		Commits: map[worktree.Ref][]string{worktree.LocalBranch{Name: "feat/a"}: {"one", "two"}},
		Origin:  map[string][]string{"main": {"upstream"}},
		Fail:    w.Fail,
	})
	meta := storetest.NewFake(storetest.FakeParams{Meta: map[string]worktree.Meta{"feat/a": {CreatedAt: createdAt}, "feat/b": {CreatedAt: createdAt}}})
	prs := []worktree.PullRequest{
		{Number: 12, URL: "https://github.com/acme/acme/pull/12", Branch: "feat/b", State: worktree.PullRequestOpen},
		{Number: 9, URL: "https://github.com/acme/acme/pull/9", Branch: "feat/a", State: worktree.PullRequestMerged},
	}

	return list.UseCase{
		Git:  git,
		Meta: meta,
		Jobs: func(context.Context) ([]run.Job, error) {
			return []run.Job{{Name: "web", WorkDir: "/repo/.trees/feat-a", Up: true}}, w.JobsErr
		},
		PullRequests: func(context.Context, []string) ([]worktree.PullRequest, error) { return prs, w.PRsErr },
		Repository:   repository.Service{Config: configtest.NewFake(configtest.FakeParams{Config: &project.Config{Worktrees: project.Worktrees{BasePath: "../.trees", BaseBranch: "main"}}, Fail: map[string]error{"Load": w.ConfigErr}}), Git: git, MainPath: mainPath, Init: &domain.FollowUp{Command: domain.CommandInit}},
	}, nil
}
