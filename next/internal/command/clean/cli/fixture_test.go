package cli

import (
	"context"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/configtest"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/shell"
	"github.com/LucasPcq/wtm/next/internal/adapter/storetest"
	"github.com/LucasPcq/wtm/next/internal/command/clean"
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

type session = form.Session[clean.Request, clean.Questions, clean.Outcome]

type sessionParams struct {
	App     form.UseCase[clean.Request, clean.Questions, clean.Outcome]
	Request clean.Request
}

// The steps of clean's form, in the order asked, then the recap.
const (
	stepWorktrees = iota
	stepReparent
	stepForce
	stepRecap
)

const (
	mainPath    = "/repo/main"
	plain       = "feat/plain"
	parent      = "feat/parent"
	child       = "feat/child"
	dirty       = "feat/dirty"
	fingerprint = "0123456789abcdef01234567"
)

var createdAt = time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)

type seed struct {
	Env      uicli.Env // zero runs in the main worktree
	NoConfig bool
	OnClean  []project.Hook
	RunHooks func(ctx context.Context, chain env.HookChain) error
	JobsErr  error
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
	if s.NoConfig {
		config = nil
	}
	return world{
		git: gittest.NewFake(gittest.FakeParams{
			Main:   mainPath,
			Branch: "main",
			Worktrees: []worktree.Worktree{
				{Path: pathOf(plain), State: worktree.OnBranch{Branch: plain}},
				{Path: pathOf(parent), State: worktree.OnBranch{Branch: parent}},
				{Path: pathOf(child), State: worktree.OnBranch{Branch: child}},
				{Path: pathOf(dirty), State: worktree.OnBranch{Branch: dirty}},
			},
			Dirty: []string{pathOf(dirty)},
		}),
		store: storetest.NewFake(storetest.FakeParams{Meta: map[string]worktree.Meta{
			plain:  {SourceBranch: "main"},
			parent: {SourceBranch: "main"},
			child:  {SourceBranch: parent},
			dirty:  {SourceBranch: "main"},
		}}),
		config: configtest.NewFake(configtest.FakeParams{Config: config}),
		locks:  &operationtest.Locks{},
		seed:   s,
	}
}

func (w world) env() uicli.Env {
	if w.seed.Env.ProjectDir != "" {
		return w.seed.Env
	}
	return uicli.Env{Cwd: mainPath, ProjectDir: mainPath}
}

func (w world) open(context.Context, string) (clean.UseCase, error) {
	return w.usecase(), nil
}

func (w world) usecase() clean.UseCase {
	runHooks := w.seed.RunHooks
	if runHooks == nil {
		runHooks = func(context.Context, env.HookChain) error { return nil }
	}
	return clean.UseCase{
		Git:          w.git,
		Meta:         w.store,
		Jobs:         func(context.Context) ([]run.Job, error) { return nil, w.seed.JobsErr },
		PullRequests: func(context.Context, []string) ([]worktree.PullRequest, error) { return nil, nil },
		Teardown:     teardown.Service{Git: w.git, Meta: w.store, EnvFiles: w.store, StateDir: "/repo/main/.git/wtm", RunHooks: runHooks, Shield: shell.Shield},
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
