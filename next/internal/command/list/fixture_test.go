package list_test

import (
	"context"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/configtest"
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

// A seed is a repository to list: its git, its meta records (and those that do not read), its config's error, the daemon's jobs and GitHub's pull requests, each with the error reading it.
type seed struct {
	Git        gittest.FakeParams
	Meta       map[string]worktree.Meta
	Unreadable []string
	ConfigErr  error
	Base       string // the base branch, main when empty
	Jobs       []run.Job
	JobsErr    error
	PRs        []worktree.PullRequest
	PRsErr     error
}

// A world is list's use case over the fakes a seed describes, with the branches GitHub was asked of, once per ask.
type world struct {
	usecase list.UseCase
	git     *gittest.Fake
	meta    *storetest.Fake
	asked   *[][]string
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
	meta := storetest.NewFake(storetest.FakeParams{Meta: s.Meta, Unreadable: s.Unreadable})
	var asked [][]string
	config := project.Config{Worktrees: project.Worktrees{BasePath: "../.trees", BaseBranch: base}}
	return world{
		usecase: list.UseCase{
			Git:  git,
			Meta: meta,
			Jobs: func(context.Context) ([]run.Job, error) { return s.Jobs, s.JobsErr },
			PullRequests: func(_ context.Context, branches []string) ([]worktree.PullRequest, error) {
				asked = append(asked, branches)
				return s.PRs, s.PRsErr
			},
			Repository: repository.Service{Config: configtest.NewFake(configtest.FakeParams{Config: &config, Fail: map[string]error{"Load": s.ConfigErr}}), Git: git, MainPath: mainPath},
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
