package cli

import (
	"context"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/configtest"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/shell"
	"github.com/LucasPcq/wtm/next/internal/command/fastforward"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

type sessionParams struct {
	App     form.UseCase[fastforward.Request, fastforward.Questions, fastforward.Outcome]
	Request fastforward.Request
}

// The steps of fast-forward's form, in the order asked, then the recap.
const (
	stepWorktrees = iota
	stepAll
	stepForce
	stepNoFetch
	stepRecap
)

const (
	mainPath    = "/repo/main"
	behind      = "feat/behind"
	dirty       = "feat/dirty"
	latest      = "feat/latest"
	diverged    = "feat/diverged"
	fingerprint = "0123456789abcdef01234567"
)

var createdAt = time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)

type world struct {
	git    *gittest.Fake
	config *configtest.Fake
	locks  *operationtest.Locks
}

func pathOf(branch string) string {
	return "/repo/wt-" + worktree.PathSegment(branch)
}

// newWorld is a repository whose main worktree and feat/behind are behind origin, feat/dirty too with an uncommitted change, feat/latest up to date and feat/diverged diverged from it.
func newWorld() world {
	upstream := []string{"upstream"}
	return world{
		git: gittest.NewFake(gittest.FakeParams{
			Main:   mainPath,
			Branch: "main",
			Remote: []string{"main", behind, dirty, latest, diverged},
			Worktrees: []worktree.Worktree{
				{Path: pathOf(behind), State: worktree.OnBranch{Branch: behind}},
				{Path: pathOf(dirty), State: worktree.OnBranch{Branch: dirty}},
				{Path: pathOf(latest), State: worktree.OnBranch{Branch: latest}},
				{Path: pathOf(diverged), State: worktree.OnBranch{Branch: diverged}},
			},
			Dirty:   []string{pathOf(dirty)},
			Changes: map[string][]worktree.Change{pathOf(dirty): {{Path: "notes.txt", Status: worktree.ChangeUntracked}}},
			Commits: map[worktree.Ref][]string{
				worktree.RemoteBranch{Name: "main"}:   upstream,
				worktree.RemoteBranch{Name: behind}:   upstream,
				worktree.RemoteBranch{Name: dirty}:    upstream,
				worktree.RemoteBranch{Name: diverged}: upstream,
				worktree.LocalBranch{Name: diverged}:  {"local work"},
			},
		}),
		config: configtest.NewFake(configtest.FakeParams{Config: &project.Config{Worktrees: project.Worktrees{BasePath: project.DefaultBasePath, BaseBranch: project.DefaultBaseBranch}}}),
		locks:  &operationtest.Locks{},
	}
}

func (w world) env() uicli.Env {
	return uicli.Env{Cwd: mainPath, ProjectDir: mainPath}
}

func (w world) open(context.Context, string) (fastforward.UseCase, error) {
	return w.usecase(), nil
}

func (w world) usecase() fastforward.UseCase {
	return fastforward.UseCase{
		Git:        w.git,
		Shield:     shell.Shield,
		Repository: repository.Service{Config: w.config, Git: w.git, MainPath: mainPath, Init: &domain.FollowUp{Command: "init"}},
		Operations: operation.Service{
			Locker:    w.locks,
			Publisher: w.locks,
			NewOpID:   func() string { return "op-1" },
			Now:       func() time.Time { return createdAt },
		},
		Provision: provision.Service{Git: w.git},
	}
}
