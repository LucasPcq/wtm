package resolve_test

import (
	"github.com/LucasPcq/wtm/next/internal/adapter/configtest"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/command/resolve"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

const mainPath = "/repo/main"

// A seed is a repository to resolve in: its git, and the error loading its config.
type seed struct {
	Git       gittest.FakeParams
	ConfigErr error
}

// A world is resolve's use case over the fakes a seed describes.
type world struct {
	usecase resolve.UseCase
	git     *gittest.Fake
}

func newWorld(s seed) world {
	if s.Git.Main == "" {
		s.Git.Main, s.Git.Branch = mainPath, "main"
	}

	git := gittest.NewFake(s.Git)
	config := project.Config{Worktrees: project.Worktrees{BasePath: "../.trees", BaseBranch: "main"}}
	return world{
		usecase: resolve.UseCase{
			Git:        git,
			Repository: repository.Service{Config: configtest.NewFake(configtest.FakeParams{Config: &config, Fail: map[string]error{"Load": s.ConfigErr}}), Git: git, MainPath: mainPath, Init: &domain.FollowUp{Command: domain.CommandInit}},
		},
		git: git,
	}
}

// linked is a linked worktree under /repo/.trees on branch.
func linked(branch string) worktree.Worktree {
	return worktree.Worktree{Path: "/repo/.trees/" + branch, State: worktree.OnBranch{Branch: branch}}
}

// detached is a linked worktree on no branch.
var detached = worktree.Worktree{Path: "/repo/.trees/detached", State: worktree.Detached{}}

// several are worktrees two of which hold log in their branch.
var several = []worktree.Worktree{linked("feat/login"), linked("feat/logout"), linked("fix/nav"), detached}
