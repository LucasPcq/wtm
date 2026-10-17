package form

import (
	"context"
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

const mainPath = "/repo/main"

var createdAt = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

type seed struct {
	Remembered map[string]string
	Behind     bool
	Linked     []string
}

func newApp(s seed) *app.App {
	commits := map[branch.Ref][]string{}
	if s.Behind {
		commits[branch.RemoteBranch{Name: "main"}] = []string{"upstream main"}
	}
	var linked []worktree.Worktree
	for _, name := range s.Linked {
		linked = append(linked, worktree.Worktree{Path: "/elsewhere/" + branch.PathSegment(name), State: worktree.OnBranch{Branch: name}})
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
	return app.New(app.Deps{
		Create: create.Deps{
			Git:        git,
			LoadConfig: func() (project.Config, project.Global, error) { return config, project.Global{}, nil },
			LoadRun:    func() (run.Config, error) { return run.Config{}, nil },
			Remember:   store.Remember,
			Provision: provision.Deps{
				Git:        git,
				Meta:       store,
				EnvFiles:   store,
				RunHooks:   func(context.Context, hook.Chain) error { return nil },
				PublicPort: func(context.Context, int) int { return 0 },
				Shield:     shell.Shield,
				Now:        func() time.Time { return createdAt },
			},
			MainPath: mainPath,
			StateDir: mainPath + "/.git/wtm",
		},
		TakeLocks: func(operation.Operation) (app.Hold, error) { return held{}, nil },
		Publisher: func(operation.Operation) wtm.Report { return func(wtm.Event) {} },
		NewOpID:   func() string { return "op-1" },
		Now:       func() time.Time { return createdAt },
	})
}

type held struct{}

func (held) Grow([]operation.Lock) (bool, error) { return false, nil }
func (held) Protected() bool                     { return true }
func (held) Release()                            {}

type refusing struct {
	App
	err error
}

func (r refusing) Create(context.Context, app.CreateParams) (app.CreateOutcome, error) {
	return app.CreateOutcome{}, r.err
}
