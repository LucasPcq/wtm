package tree_test

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/storetest"
	"github.com/LucasPcq/wtm/next/internal/command/tree"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

const mainPath = "/repo/main"

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

// A seed is a repository to read as a tree: its git, its meta records, its config's error, the daemon's jobs and GitHub's pull requests, each with the error reading it.
type seed struct {
	Git        gittest.FakeParams
	Meta       map[string]worktree.Meta
	Unreadable []string // branches whose record does not read
	ConfigErr  error
	Base       string // the base branch, main when empty
	Jobs       []run.Job
	JobsErr    error
	PRs        []worktree.PullRequest
	PRsErr     error
}

// A world is tree's use case over the fakes a seed describes, with the branches GitHub was asked of, once per ask.
type world struct {
	usecase tree.UseCase
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
		usecase: tree.UseCase{
			Git:  git,
			Meta: meta,
			Jobs: func(context.Context) ([]run.Job, error) { return s.Jobs, s.JobsErr },
			PullRequests: func(_ context.Context, branches []string) ([]worktree.PullRequest, error) {
				asked = append(asked, branches)
				return s.PRs, s.PRsErr
			},
			Repository: repository.Service{Config: configured{config: config, err: s.ConfigErr}, Git: git, MainPath: mainPath},
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

// from is the meta record of a worktree created from source at createdAt.
func from(source, createdAt string) worktree.Meta {
	return worktree.Meta{SourceBranch: source, CreatedAt: createdAt}
}

// commits are a branch's commits after the root: its local branch's in the fake.
func commits(branches map[string][]string) map[worktree.Ref][]string {
	refs := map[worktree.Ref][]string{}
	for branch, list := range branches {
		refs[worktree.LocalBranch{Name: branch}] = list
	}
	return refs
}
