package provision_test

import (
	"context"
	"errors"
	"maps"
	"path/filepath"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/shell"
	"github.com/LucasPcq/wtm/next/internal/adapter/store"
	"github.com/LucasPcq/wtm/next/internal/adapter/storetest"
	"github.com/LucasPcq/wtm/next/internal/app/provision"
	"github.com/LucasPcq/wtm/next/internal/wtm/branch"
	"github.com/LucasPcq/wtm/next/internal/wtm/dotenv"
	"github.com/LucasPcq/wtm/next/internal/wtm/project"
	"github.com/LucasPcq/wtm/next/internal/wtm/run"
	"github.com/LucasPcq/wtm/next/internal/wtm/worktree"
)

const (
	branchName = "feat/x"
	existing   = "feature"
	mainEnv    = "PORT=3000\n"
)

var createdAt = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

type repo interface {
	provision.Git
	Branches(ctx context.Context) (branch.Branches, error)
}

type world struct {
	root     string
	main     string
	stateDir string
	git      repo
	meta     provision.Meta
	deps     provision.Deps
}

func onDisk(t *testing.T) world {
	t.Helper()
	root := gittest.TempDir(t)
	main := filepath.Join(root, "main")
	repo := gittest.Build(t, gittest.FakeParams{Main: main, Branch: "main", Local: []string{existing}})
	writeFile(t, filepath.Join(main, ".env"), mainEnv)
	stateDir := filepath.Join(main, ".git", "wtm")
	meta := store.Meta{StateDir: stateDir}
	return world{root: root, main: main, stateDir: stateDir, git: repo, meta: meta, deps: deps(repo, meta, store.EnvFiles{})}
}

type seeds struct {
	GitFail   map[string]error
	Worktrees []worktree.Worktree
	Store     storetest.FakeParams
}

func inMemory(t *testing.T, seed seeds) (world, *storetest.Fake) {
	t.Helper()
	root := gittest.TempDir(t)
	main := filepath.Join(root, "main")
	repo := gittest.NewFake(gittest.FakeParams{Main: main, Branch: "main", Local: []string{existing}, Worktrees: seed.Worktrees, Fail: seed.GitFail})
	seeded := seed.Store
	seeded.Files = maps.Clone(seeded.Files)
	if seeded.Files == nil {
		seeded.Files = map[string]string{}
	}
	seeded.Files[filepath.Join(main, ".env")] = mainEnv
	files := storetest.NewFake(seeded)
	return world{root: root, main: main, stateDir: filepath.Join(main, ".git", "wtm"), git: repo, meta: files, deps: deps(repo, files, files)}, files
}

func deps(git provision.Git, meta provision.Meta, env provision.EnvFiles) provision.Deps {
	return provision.Deps{
		Git:        git,
		Meta:       meta,
		EnvFiles:   env,
		RunHooks:   shell.RunHooks,
		PublicPort: func(context.Context, int) int { return 0 },
		Shield:     shell.Shield,
		Now:        func() time.Time { return createdAt },
	}
}

func (w world) params(report *recorder) provision.Params {
	return provision.Params{
		Deps:    w.deps,
		Project: provision.Project{MainPath: w.main, StateDir: w.stateDir, EnvFiles: []project.EnvFile{{Target: ".env"}}},
		Worktree: provision.Worktree{
			Branch:   branchName,
			Path:     w.path(branchName),
			Checkout: worktree.NewBranch{Name: branchName, From: branch.LocalBranch{Name: "main"}},
			Source:   "main",
			Strategy: project.EnvMain,
		},
		Report: report.add,
	}
}

func (w world) existing(report *recorder) provision.Params {
	params := w.params(report)
	params.Worktree.Branch = existing
	params.Worktree.Path = w.path(existing)
	params.Worktree.Checkout = worktree.ExistingBranch{Name: existing}
	return params
}

func (w world) path(name string) string {
	return filepath.Join(w.root, filepath.Base(name))
}

func portLinked(params provision.Params) provision.Params {
	params.Project.Run = run.Config{
		Jobs:     []run.Job{{Name: "web", Kind: run.Service, Cmd: "npm start", Ports: map[string]int{"http": 3000}}},
		EnvPorts: []run.EnvPortLink{{File: ".env", Key: "PORT", Job: "web", Port: "http"}},
	}
	return params
}

func composed(params provision.Params) provision.Params {
	params.Project.Run = run.Config{
		Jobs: []run.Job{{Name: "db", Kind: run.Service, Cmd: "docker compose up -d", Ports: map[string]int{"pg": 5432}}},
	}
	return params
}

func hooked(params provision.Params, commands ...string) provision.Params {
	for _, command := range commands {
		params.Project.OnCreate = append(params.Project.OnCreate, project.Hook{Cmd: command})
	}
	return params
}

type unlocked struct{ provision.Meta }

func (unlocked) LockOrdinals(_ context.Context, do func() error) (bool, error) {
	return false, do()
}

type unreadable struct{ provision.EnvFiles }

func (unreadable) Read(string) ([]dotenv.Line, error) {
	return nil, errors.New("read .env: permission denied")
}
