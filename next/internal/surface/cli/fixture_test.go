package cli

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/shell"
	"github.com/LucasPcq/wtm/next/internal/adapter/storetest"
	"github.com/LucasPcq/wtm/next/internal/app"
	"github.com/LucasPcq/wtm/next/internal/app/create"
	"github.com/LucasPcq/wtm/next/internal/app/provision"
	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/branch"
	"github.com/LucasPcq/wtm/next/internal/wtm/hook"
	"github.com/LucasPcq/wtm/next/internal/wtm/operation"
	"github.com/LucasPcq/wtm/next/internal/wtm/project"
	"github.com/LucasPcq/wtm/next/internal/wtm/run"
	"github.com/LucasPcq/wtm/next/internal/wtm/worktree"
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
	RunHooks   func(ctx context.Context, chain hook.Chain) error
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
	commits := map[branch.Ref][]string{}
	if s.Behind {
		commits[branch.RemoteBranch{Name: "main"}] = []string{"upstream main"}
	}
	var linked []worktree.Worktree
	for _, name := range s.Linked {
		linked = append(linked, worktree.Worktree{Path: filepath.Join("/elsewhere", branch.PathSegment(name)), State: worktree.OnBranch{Branch: name}})
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
	return Project{App: app.New(w.deps()), Cwd: mainPath}, nil
}

func (w world) deps() app.Deps {
	runHooks := w.seed.RunHooks
	if runHooks == nil {
		runHooks = func(context.Context, hook.Chain) error { return nil }
	}
	return app.Deps{
		Create: create.Deps{
			Git: w.git,
			LoadConfig: func() (project.Config, project.Global, error) {
				config, found := w.store.Config()
				if !found {
					return project.Config{}, project.Global{}, project.ErrConfigNotFound
				}
				return config, project.Global{}, nil
			},
			LoadRun:  func() (run.Config, error) { return run.Config{}, nil },
			Remember: w.store.Remember,
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
		},
		TakeLocks: func(operation.Operation) (app.Hold, error) {
			if w.seed.Locks != nil {
				return nil, w.seed.Locks
			}
			return held{}, nil
		},
		Publisher: func(operation.Operation) wtm.Report { return func(wtm.Event) {} },
		NewOpID:   func() string { return "op-1" },
		Now:       func() time.Time { return createdAt },
		Surface:   "cli",
	}
}

type held struct{}

func (held) Grow([]operation.Lock) (bool, error) { return false, nil }
func (held) Protected() bool                     { return true }
func (held) Release()                            {}

func hookFails(context.Context, hook.Chain) error {
	return errors.New("hook failed: exit status 3")
}
