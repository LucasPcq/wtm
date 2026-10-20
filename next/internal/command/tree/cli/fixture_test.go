package cli

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/storetest"
	"github.com/LucasPcq/wtm/next/internal/command/tree"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
)

const mainPath = "/repo/main"

// configured is a project's config as repository.Service loads it, or the error loading it.
type configured struct {
	err error
}

func (c configured) Load() (project.Config, project.Global, error) {
	return project.Config{Worktrees: project.Worktrees{BasePath: "../.trees", BaseBranch: "main"}}, project.Global{}, c.err
}

func (configured) LoadRun() (project.Run, error) {
	return project.Run{}, nil
}

// A world is a repository of every kind of node tree draws: a stack of three levels with a job up and commits ahead, a fan-out whose second branch lacks a commit of main and is locked, a parent with no worktree, a cycle, a rebasing worktree, one whose directory is gone and a detached one; Worktrees, when set, replaces them, on branches with no commit and no origin.
type world struct {
	ConfigErr error
	PRsErr    error
	JobsErr   error
	Worktrees []worktree.Worktree
	Meta      map[string]worktree.Meta
	Fail      map[string]error // git's failures, by method
}

func (w world) env() uicli.Env {
	return uicli.Env{Cwd: mainPath, ProjectDir: mainPath}
}

func (w world) open(context.Context, string) (tree.UseCase, error) {
	worktrees, meta := w.Worktrees, w.Meta
	commits := map[worktree.Ref][]string{}
	var remote []string
	var origin map[string][]string
	if worktrees == nil {
		worktrees = []worktree.Worktree{
			{Path: "/repo/.trees/feat-a", State: worktree.OnBranch{Branch: "feat/a"}},
			{Path: "/repo/.trees/feat-a-ui", State: worktree.OnBranch{Branch: "feat/a-ui"}},
			{Path: "/repo/.trees/feat-a-ui-copy", State: worktree.OnBranch{Branch: "feat/a-ui-copy"}},
			{Path: "/repo/.trees/feat-b", State: worktree.OnBranch{Branch: "feat/b"}, Locked: true},
			{Path: "/repo/.trees/feat-x", State: worktree.OnBranch{Branch: "feat/x"}},
			{Path: "/repo/.trees/y", State: worktree.OnBranch{Branch: "y"}},
			{Path: "/repo/.trees/z", State: worktree.OnBranch{Branch: "z"}},
			{Path: "/repo/.trees/rebasing", State: worktree.Rebasing{Branch: "rebasing"}},
			{Path: "/repo/.trees/gone", State: worktree.OnBranch{Branch: "feat/gone"}},
			{Path: "/repo/.trees/detached", State: worktree.Detached{}},
		}
		meta = map[string]worktree.Meta{
			"feat/a":         {SourceBranch: "main", CreatedAt: "2026-10-01T00:00:00Z"},
			"feat/a-ui":      {SourceBranch: "feat/a", CreatedAt: "2026-10-02T00:00:00Z"},
			"feat/a-ui-copy": {SourceBranch: "feat/a-ui", CreatedAt: "2026-10-03T00:00:00Z"},
			"feat/b":         {SourceBranch: "main", CreatedAt: "2026-10-04T00:00:00Z"},
			"feat/x":         {SourceBranch: "feat/login", CreatedAt: "2026-10-05T00:00:00Z"},
			"y":              {SourceBranch: "z", CreatedAt: "2026-10-06T00:00:00Z"},
			"z":              {SourceBranch: "y", CreatedAt: "2026-10-07T00:00:00Z"},
			"rebasing":       {SourceBranch: "main", CreatedAt: "2026-10-08T00:00:00Z"},
			"feat/gone":      {SourceBranch: "main", CreatedAt: "2026-10-09T00:00:00Z"},
		}
		commits = map[worktree.Ref][]string{
			worktree.LocalBranch{Name: "main"}:           {"m1"},
			worktree.LocalBranch{Name: "feat/a"}:         {"m1", "a1", "a2"},
			worktree.LocalBranch{Name: "feat/a-ui"}:      {"m1", "a1", "a2", "u1"},
			worktree.LocalBranch{Name: "feat/a-ui-copy"}: {"m1", "a1", "a2", "u1"},
			worktree.LocalBranch{Name: "feat/b"}:         {"b1"},
			worktree.LocalBranch{Name: "rebasing"}:       {"m1"},
		}
		remote, origin = []string{"main", "feat/a"}, map[string][]string{"main": {"m1", "upstream"}}
	}
	git := gittest.NewFake(gittest.FakeParams{
		Main: mainPath, Branch: "main", Remote: remote,
		Worktrees: worktrees,
		Gone:      []string{"/repo/.trees/gone"},
		Dirty:     []string{"/repo/.trees/feat-a", "/repo/.trees/rebasing"},
		Commits:   commits,
		Origin:    origin,
		Fail:      w.Fail,
	})
	store := storetest.NewFake(storetest.FakeParams{Meta: meta})
	prs := []worktree.PullRequest{
		{Number: 12, URL: "https://github.com/acme/acme/pull/12", Branch: "feat/a", State: worktree.PullRequestOpen},
		{Number: 9, URL: "https://github.com/acme/acme/pull/9", Branch: "feat/b", State: worktree.PullRequestMerged},
	}

	return tree.UseCase{
		Git:  git,
		Meta: store,
		Jobs: func(context.Context) ([]run.Job, error) {
			return []run.Job{{Name: "web", WorkDir: "/repo/.trees/feat-a", Up: true}}, w.JobsErr
		},
		PullRequests: func(context.Context, []string) ([]worktree.PullRequest, error) { return prs, w.PRsErr },
		Repository:   repository.Service{Config: configured{err: w.ConfigErr}, Git: git, MainPath: mainPath},
	}, nil
}
