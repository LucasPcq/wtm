package cli

import (
	"cmp"
	"context"

	"github.com/LucasPcq/wtm/next/internal/adapter/configtest"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/command/resolve"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
)

const mainPath = "/repo/main"

// A world is a repository with feat/login, feat/logout and fix/nav beside the main worktree, and one on no branch; Cwd is where the command runs.
type world struct {
	Cwd       string
	ConfigErr error
	Gone      bool // fix/nav's directory is gone
}

func (w world) env() uicli.Env {
	cwd := cmp.Or(w.Cwd, mainPath)
	return uicli.Env{Cwd: cwd, ProjectDir: cwd}
}

func (w world) open(context.Context, string) (resolve.UseCase, error) {
	var gone []string
	if w.Gone {
		gone = []string{"/repo/.trees/fix-nav"}
	}
	git := gittest.NewFake(gittest.FakeParams{
		Main: mainPath, Branch: "main",
		Worktrees: []worktree.Worktree{
			{Path: "/repo/.trees/feat-login", State: worktree.OnBranch{Branch: "feat/login"}},
			{Path: "/repo/.trees/feat-logout", State: worktree.OnBranch{Branch: "feat/logout"}},
			{Path: "/repo/.trees/fix-nav", State: worktree.OnBranch{Branch: "fix/nav"}},
			{Path: "/repo/.trees/detached", State: worktree.Detached{}},
		},
		Gone: gone,
	})

	return resolve.UseCase{
		Git: git,
		Repository: repository.Service{Config: configtest.NewFake(configtest.FakeParams{
			Config: &project.Config{Worktrees: project.Worktrees{BasePath: "../.trees", BaseBranch: "main"}},
			Fail:   map[string]error{"Load": w.ConfigErr},
		}), Git: git, MainPath: mainPath, Init: &domain.FollowUp{Command: domain.CommandInit}},
	}, nil
}
