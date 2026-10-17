package cli

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain/env"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/shell"
	"github.com/LucasPcq/wtm/next/internal/adapter/storetest"
	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/provision"

	"github.com/LucasPcq/wtm/next/internal/domain/project"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

const (
	mainPath    = "/repo/main"
	stateDir    = "/repo/main/.git/wtm"
	branchName  = "feat/x"
	fingerprint = "0123456789abcdef01234567"
)

var createdAt = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

type seed struct {
	Remembered map[string]string
	OnCreate   []project.Hook
	Linked     []string
	Behind     bool
	RunHooks   func(ctx context.Context, chain env.HookChain) error
	Locks      error
	Open       error
	GitFails   map[string]error
	StoreFails map[string]error
}

type world struct {
	git   *gittest.Fake
	store *storetest.Fake
	seed  seed
}

func newWorld(s seed) world {
	commits := map[worktree.Ref][]string{}
	if s.Behind {
		commits[worktree.RemoteBranch{Name: "main"}] = []string{"upstream main"}
	}
	var linked []worktree.Worktree
	for _, name := range s.Linked {
		linked = append(linked, worktree.Worktree{Path: filepath.Join("/elsewhere", worktree.PathSegment(name)), State: worktree.OnBranch{Branch: name}})
	}
	config := project.Config{
		Worktrees:  project.Worktrees{BasePath: project.DefaultBasePath, BaseBranch: project.DefaultBaseBranch},
		Env:        project.Env{Strategy: project.EnvExample},
		OnCreate:   s.OnCreate,
		Remembered: s.Remembered,
	}
	return world{
		git:   gittest.NewFake(gittest.FakeParams{Main: mainPath, Branch: "main", Remote: []string{"main"}, Worktrees: linked, Commits: commits, Fail: s.GitFails}),
		store: storetest.NewFake(storetest.FakeParams{Config: &config, Fail: s.StoreFails}),
		seed:  s,
	}
}

func (w world) params(args []string) Params {
	return Params{Version: "1.2.3", Args: args, Open: w.open, KillCancelled: shell.KillCancelled, AwaitShields: shell.AwaitShields}
}

func (w world) open(context.Context) (Project, error) {
	if w.seed.Open != nil {
		return Project{}, w.seed.Open
	}
	return Project{Create: w.usecase(), Cwd: mainPath}, nil
}

func (w world) usecase() create.UseCase {
	runHooks := w.seed.RunHooks
	if runHooks == nil {
		runHooks = func(context.Context, env.HookChain) error { return nil }
	}
	return create.UseCase{
		Git:     w.git,
		Config:  stored{w.store},
		Answers: w.store,
		Operations: operation.Service{
			Locker:    locker{err: w.seed.Locks},
			Publisher: locker{},
			NewOpID:   func() string { return "op-1" },
			Now:       func() time.Time { return createdAt },
		},
		Provision: provision.Deps{
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

type locker struct {
	err error
}

func (l locker) Take(domain.Operation) (operation.Hold, error) {
	if l.err != nil {
		return nil, l.err
	}
	return held{}, nil
}

func (locker) Open(domain.Operation) domain.Report { return func(domain.Event) {} }

type held struct{}

func (held) Grow([]domain.Lock) (bool, error) { return false, nil }
func (held) Protected() bool                  { return true }
func (held) Release()                         {}

func hookFails(context.Context, env.HookChain) error {
	return errors.New("hook failed: exit status 3")
}
