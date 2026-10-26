package cli

import (
	"context"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/configtest"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/shell"
	"github.com/LucasPcq/wtm/next/internal/adapter/storetest"
	"github.com/LucasPcq/wtm/next/internal/command/prune"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
	"github.com/LucasPcq/wtm/next/internal/service/teardown"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

type sessionParams struct {
	App     form.UseCase[prune.Request, prune.Questions, prune.Outcome]
	Request prune.Request
}

// The steps of prune's form, in the order asked, then the recap.
const (
	stepWorktrees = iota
	stepReparent
	stepForce
	stepRecap = 7
)

const (
	mainPath    = "/repo/main"
	merged      = "feat/merged" // its pull request merged at its tip
	child       = "feat/child"  // created from feat/merged
	dirty       = "feat/dirty"  // its pull request merged at its tip, uncommitted changes
	gone        = "feat/gone"   // tracking a branch origin no longer has
	plain       = "feat/plain"  // no pull request
	fingerprint = "0123456789abcdef01234567"
)

var createdAt = time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)

type seed struct {
	OnClean  []project.Hook
	RunHooks func(ctx context.Context, chain env.HookChain) error
	PRErr    error
	NoPRs    bool
}

type world struct {
	git    *gittest.Fake
	store  *storetest.Fake
	config *configtest.Fake
	locks  *operationtest.Locks
	seed   seed
}

func pathOf(branch string) string {
	return "/repo/wt-" + worktree.PathSegment(branch)
}

func newWorld(s seed) world {
	config := &project.Config{Worktrees: project.Worktrees{BasePath: project.DefaultBasePath, BaseBranch: project.DefaultBaseBranch}, OnClean: s.OnClean}
	return world{
		git: gittest.NewFake(gittest.FakeParams{
			Main:     mainPath,
			Branch:   "main",
			Remote:   []string{"main"},
			Tracking: []string{gone},
			Worktrees: []worktree.Worktree{
				{Path: pathOf(merged), State: worktree.OnBranch{Branch: merged}},
				{Path: pathOf(child), State: worktree.OnBranch{Branch: child}},
				{Path: pathOf(dirty), State: worktree.OnBranch{Branch: dirty}},
				{Path: pathOf(gone), State: worktree.OnBranch{Branch: gone}},
				{Path: pathOf(plain), State: worktree.OnBranch{Branch: plain}},
			},
			Dirty: []string{pathOf(dirty)},
			Commits: map[worktree.Ref][]string{
				worktree.LocalBranch{Name: merged}: {"merged work"},
				worktree.LocalBranch{Name: dirty}:  {"dirty work"},
			},
		}),
		store: storetest.NewFake(storetest.FakeParams{Meta: map[string]worktree.Meta{
			merged: {SourceBranch: "main"},
			child:  {SourceBranch: merged},
			dirty:  {SourceBranch: "main"},
			gone:   {SourceBranch: "main"},
			plain:  {SourceBranch: "main"},
		}}),
		config: configtest.NewFake(configtest.FakeParams{Config: config}),
		locks:  &operationtest.Locks{},
		seed:   s,
	}
}

func (w world) env() uicli.Env {
	return uicli.Env{Cwd: mainPath, ProjectDir: mainPath}
}

func (w world) open(context.Context, string) (prune.UseCase, error) {
	return w.usecase(), nil
}

func (w world) pullRequests(context.Context, []string) ([]worktree.PullRequest, error) {
	if w.seed.PRErr != nil || w.seed.NoPRs {
		return nil, w.seed.PRErr
	}
	return []worktree.PullRequest{
		{Number: 12, URL: "https://github.com/acme/web/pull/12", Branch: merged, State: worktree.PullRequestMerged, Head: "merged work"},
		{Number: 14, URL: "https://github.com/acme/web/pull/14", Branch: dirty, State: worktree.PullRequestMerged, Head: "dirty work"},
	}, nil
}

func (w world) usecase() prune.UseCase {
	runHooks := w.seed.RunHooks
	if runHooks == nil {
		runHooks = func(context.Context, env.HookChain) error { return nil }
	}
	return prune.UseCase{
		Git:          w.git,
		Meta:         w.store,
		Jobs:         func(context.Context) ([]run.Job, error) { return nil, nil },
		PullRequests: w.pullRequests,
		Teardown:     teardown.Service{Git: w.git, Meta: w.store, EnvFiles: w.store, StateDir: "/repo/main/.git/wtm", Gone: repository.GoneIn(w.git), RunHooks: runHooks, Shield: shell.Shield},
		Repository:   repository.Service{Config: w.config, Git: w.git, MainPath: mainPath, Init: &domain.FollowUp{Command: "init"}},
		Operations: operation.Service{
			Locker:    w.locks,
			Publisher: w.locks,
			NewOpID:   func() string { return "op-1" },
			Now:       func() time.Time { return createdAt },
		},
	}
}

func hookFails(context.Context, env.HookChain) error {
	return &env.HookFailure{Cmd: "make down", Err: errHook}
}
