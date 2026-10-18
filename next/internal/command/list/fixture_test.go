package list_test

import (
	"context"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/storetest"
	"github.com/LucasPcq/wtm/next/internal/command/list"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

const mainPath = "/repo/main"

var created = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

// configured is a project's config as repository.Service loads it, or the error loading it.
type configured struct {
	config project.Config
	err    error
}

func (c configured) Load() (project.Config, project.Global, error) {
	return c.config, project.Global{}, c.err
}

func (configured) LoadRun() (project.Run, error) {
	return project.Run{}, nil
}

// A seed is a repository to list: its git, its meta records, its config's error, the daemon's jobs and GitHub's pull requests, each with the error reading it.
type seed struct {
	Git       gittest.FakeParams
	Meta      map[string]worktree.Meta
	ConfigErr error
	Base      string // the base branch, main when empty
	Jobs      []run.Job
	JobsErr   error
	PRs       worktree.PullRequests
	PRsErr    error
}

// A world is list's use case over the fakes a seed describes, with how many times GitHub was asked.
type world struct {
	usecase list.UseCase
	git     *gittest.Fake
	meta    *storetest.Fake
	asked   *int
}

func newWorld(s seed) world {
	if s.Git.Main == "" {
		s.Git.Main, s.Git.Branch = mainPath, "main"
	}
	base := s.Base
	if base == "" {
		base = "main"
	}

	git := gittest.NewFake(s.Git)
	meta := storetest.NewFake(storetest.FakeParams{Meta: s.Meta})
	asked := 0
	config := project.Config{Worktrees: project.Worktrees{BasePath: "../.trees", BaseBranch: base}}
	return world{
		usecase: list.UseCase{
			Git:  git,
			Meta: meta,
			Jobs: func(context.Context) ([]run.Job, error) { return s.Jobs, s.JobsErr },
			PullRequests: func(context.Context) (worktree.PullRequests, error) {
				asked++
				return s.PRs, s.PRsErr
			},
			Repository: repository.Service{Config: configured{config: config, err: s.ConfigErr}, Git: git, MainPath: mainPath},
			Commands: func(action list.Action, detail list.Detail) []string {
				return []string{"wtm", string(action), detail.Branch}
			},
		},
		git:   git,
		meta:  meta,
		asked: &asked,
	}
}

// linked is a linked worktree under /repo/.trees on branch.
func linked(branch string) worktree.Worktree {
	return worktree.Worktree{Path: "/repo/.trees/" + branch, State: worktree.OnBranch{Branch: branch}}
}

// snapshotOf is git's snapshot with its worktrees replaced by worktrees, for a layout the fake does not build.
type snapshotOf struct {
	git       *gittest.Fake
	worktrees []worktree.Worktree
}

func (s snapshotOf) Snapshot(ctx context.Context) (worktree.Snapshot, error) {
	snapshot, err := s.git.Snapshot(ctx)
	snapshot.Worktrees = s.worktrees
	return snapshot, err
}
