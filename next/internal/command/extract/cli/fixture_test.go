package cli

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/shell"
	"github.com/LucasPcq/wtm/next/internal/adapter/storetest"
	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
)

const (
	mainPath    = "/repo/main"
	stateDir    = "/repo/main/.git/wtm"
	sourcePath  = "/repo/wt-feat-login"
	otherPath   = "/repo/wt-feat-other"
	source      = "feat/login"
	other       = "feat/other"
	split       = "feat/split"
	fingerprint = "0123456789abcdef01234567"
)

var createdAt = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

var changed = []worktree.Change{
	{Path: "apps/api/a.txt", Status: worktree.ChangeModified},
	{Path: "apps/api/new.txt", Status: worktree.ChangeUntracked},
	{Path: "DOC.md", Status: worktree.ChangeRenamed, OrigPath: "README.md"},
	{Path: "old.txt", Status: worktree.ChangeDeleted},
}

type seed struct {
	Clean     bool
	Conflicts map[string]worktree.ConflictKind
	NoConfig  bool
	OnCreate  []project.Hook
	RunHooks  func(ctx context.Context, chain env.HookChain) error
}

type world struct {
	git   *gittest.Fake
	store *storetest.Fake
	seed  seed
}

func newWorld(s seed) world {
	changes := map[string][]worktree.Change{sourcePath: slices.Clone(changed)}
	if s.Clean {
		changes = nil
	}
	config := &project.Config{
		Worktrees: project.Worktrees{BasePath: project.DefaultBasePath, BaseBranch: project.DefaultBaseBranch},
		Env:       project.Env{Strategy: project.EnvExample},
		OnCreate:  s.OnCreate,
	}
	if s.NoConfig {
		config = nil
	}
	return world{
		git: gittest.NewFake(gittest.FakeParams{
			Main:   mainPath,
			Branch: "main",
			Remote: []string{"main"},
			Worktrees: []worktree.Worktree{
				{Path: sourcePath, State: worktree.OnBranch{Branch: source}},
				{Path: otherPath, State: worktree.OnBranch{Branch: other}},
			},
			Changes:   changes,
			Conflicts: s.Conflicts,
		}),
		store: storetest.NewFake(storetest.FakeParams{Config: config}),
		seed:  s,
	}
}

func (w world) open(context.Context) (Project, error) {
	return Project{UseCase: w.usecase(), Cwd: mainPath}, nil
}

func (w world) usecase() extract.UseCase {
	runHooks := w.seed.RunHooks
	if runHooks == nil {
		runHooks = func(context.Context, env.HookChain) error { return nil }
	}
	return extract.UseCase{
		Git:     w.git,
		Config:  stored{w.store},
		Answers: w.store,
		Meta:    w.store,
		Shield:  shell.Shield,
		Operations: operation.Service{
			Locker:    locker{},
			Publisher: locker{},
			NewOpID:   func() string { return "op-1" },
			Now:       func() time.Time { return createdAt },
		},
		Provision: provision.Service{
			Git:        w.git,
			Meta:       w.store,
			EnvFiles:   w.store,
			RunHooks:   runHooks,
			PublicPort: func(context.Context, int) int { return 0 },
			Shield:     shell.Shield,
			Now:        func() time.Time { return createdAt },
		},
		MainPath: mainPath,
		StateDir: stateDir,
	}
}

type stored struct {
	*storetest.Fake
}

func (s stored) Load() (project.Config, project.Global, error) {
	config, found := s.Config()
	if !found {
		return project.Config{}, project.Global{}, project.ErrConfigNotFound
	}
	return config, project.Global{}, nil
}

func (stored) LoadRun() (project.Run, error) { return project.Run{}, nil }

type locker struct{}

func (locker) Take(domain.Operation) (operation.Hold, error) { return held{}, nil }
func (locker) Open(domain.Operation) domain.Report           { return func(domain.Event) {} }

type held struct{}

func (held) Grow([]domain.Lock) (bool, error) { return false, nil }
func (held) Protected() bool                  { return true }
func (held) Release()                         {}

func hookFails(context.Context, env.HookChain) error {
	return errors.New("hook failed: exit status 3")
}
