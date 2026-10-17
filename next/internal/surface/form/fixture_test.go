package form

import (
	"context"
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

const mainPath = "/repo/main"

var createdAt = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

type seed struct {
	Remembered map[string]string
	Behind     bool
	Linked     []string
}

func newApp(s seed) create.UseCase {
	commits := map[worktree.Ref][]string{}
	if s.Behind {
		commits[worktree.RemoteBranch{Name: "main"}] = []string{"upstream main"}
	}
	var linked []worktree.Worktree
	for _, name := range s.Linked {
		linked = append(linked, worktree.Worktree{Path: "/elsewhere/" + worktree.PathSegment(name), State: worktree.OnBranch{Branch: name}})
	}
	var remote []string
	if s.Behind {
		remote = []string{"main"}
	}
	git := gittest.NewFake(gittest.FakeParams{Main: mainPath, Branch: "main", Local: []string{"feature"}, Remote: remote, Worktrees: linked, Commits: commits})
	config := project.Config{
		Worktrees:  project.Worktrees{BasePath: project.DefaultBasePath, BaseBranch: project.DefaultBaseBranch},
		Env:        project.Env{Strategy: project.EnvExample, Files: []project.EnvFile{{Target: ".env"}}},
		Remembered: s.Remembered,
	}
	store := storetest.NewFake(storetest.FakeParams{Config: &config})
	return create.UseCase{
		Git:     git,
		Config:  loaded{config: config},
		Answers: store,
		Operations: operation.Service{
			Locker:    held{},
			Publisher: held{},
			NewOpID:   func() string { return "op-1" },
			Now:       func() time.Time { return createdAt },
		},
		Provision: provision.Deps{
			Git:        git,
			Meta:       store,
			EnvFiles:   store,
			RunHooks:   func(context.Context, env.HookChain) error { return nil },
			PublicPort: func(context.Context, int) int { return 0 },
			Shield:     shell.Shield,
			Now:        func() time.Time { return createdAt },
		},
		MainPath: mainPath,
		StateDir: mainPath + "/.git/wtm",
	}
}

type loaded struct {
	config project.Config
}

func (l loaded) Load() (project.Config, project.Global, error) {
	return l.config, project.Global{}, nil
}
func (loaded) LoadRun() (project.Run, error) { return project.Run{}, nil }

type held struct{}

func (held) Grow([]domain.Lock) (bool, error) { return false, nil }
func (held) Protected() bool                  { return true }
func (held) Release()                         {}

func (held) Take(domain.Operation) (operation.Hold, error) { return held{}, nil }
func (held) Open(domain.Operation) domain.Report           { return func(domain.Event) {} }

type refusing struct {
	App
	err error
}

func (r refusing) Run(context.Context, create.Request, operation.Options) (create.Outcome, error) {
	return create.Outcome{}, r.err
}
