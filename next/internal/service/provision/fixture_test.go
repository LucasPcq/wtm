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
	"github.com/LucasPcq/wtm/next/internal/service/provision"

	"github.com/LucasPcq/wtm/next/internal/domain/dotenv"
	"github.com/LucasPcq/wtm/next/internal/domain/project"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

const (
	branchName = "feat/x"
	existing   = "feature"
	mainEnv    = "PORT=3000\n"
)

var createdAt = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

// repo is the git a test provisions in, read back through Branches.
type repo interface {
	provision.Git
	Branches(ctx context.Context) (worktree.Branches, error)
}

// world is a project with a main worktree on main and a local branch existing,
// and a provision.Service over it.
type world struct {
	root     string
	main     string
	stateDir string
	git      repo
	meta     provision.Meta
	service  provision.Service
}

// onDisk builds the world in a real repository with the real meta and env stores.
func onDisk(t *testing.T) world {
	t.Helper()
	root := gittest.TempDir(t)
	main := filepath.Join(root, "main")
	repo := gittest.Build(t, gittest.FakeParams{Main: main, Branch: "main", Local: []string{existing}})
	writeFile(t, filepath.Join(main, ".env"), mainEnv)
	stateDir := filepath.Join(main, ".git", "wtm")
	meta := store.Meta{StateDir: stateDir}
	return world{root: root, main: main, stateDir: stateDir, git: repo, meta: meta, service: service(repo, meta, store.EnvFiles{})}
}

// seeds is what an in-memory world starts with: git calls that fail, linked
// worktrees and stored files.
type seeds struct {
	GitFail   map[string]error
	Worktrees []worktree.Worktree
	Store     storetest.FakeParams
}

// inMemory builds the world over the git and store fakes, the main .env
// seeded, and returns the store fake to read back or fail.
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
	return world{root: root, main: main, stateDir: filepath.Join(main, ".git", "wtm"), git: repo, meta: files, service: service(repo, files, files)}, files
}

func service(git provision.Git, meta provision.Meta, env provision.EnvFiles) provision.Service {
	return provision.Service{
		Git:        git,
		Meta:       meta,
		EnvFiles:   env,
		RunHooks:   shell.RunHooks,
		PublicPort: func(context.Context, int) int { return 0 },
		Shield:     shell.Shield,
		Now:        func() time.Time { return createdAt },
	}
}

// params provisions branchName as a new branch from main, its .env copied from
// main, its events recorded by report.
func (w world) params(report *recorder) provision.Params {
	return provision.Params{
		Project: provision.Project{MainPath: w.main, StateDir: w.stateDir, EnvFiles: []project.EnvFile{{Target: ".env"}}},
		Worktree: provision.Worktree{
			Branch:   branchName,
			Path:     w.path(branchName),
			Checkout: worktree.NewBranch{Name: branchName, From: worktree.LocalBranch{Name: "main"}},
			Source:   "main",
			Strategy: project.EnvMain,
		},
		Report: report.add,
	}
}

// existing provisions the existing local branch instead of a new one.
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

// portLinked adds a run.toml whose web job's port is linked into .env.
func portLinked(params provision.Params) provision.Params {
	params.Project.Run = project.Run{
		Jobs:     []project.Job{{Name: "web", Kind: project.Service, Cmd: "npm start", Ports: map[string]int{"http": 3000}}},
		EnvPorts: []project.EnvPortLink{{File: ".env", Key: "PORT", Job: "web", Port: "http"}},
	}
	return params
}

// composed adds a run.toml with a docker compose job and no env link.
func composed(params provision.Params) provision.Params {
	params.Project.Run = project.Run{
		Jobs: []project.Job{{Name: "db", Kind: project.Service, Cmd: "docker compose up -d", Ports: map[string]int{"pg": 5432}}},
	}
	return params
}

func hooked(params provision.Params, commands ...string) provision.Params {
	for _, command := range commands {
		params.Project.OnCreate = append(params.Project.OnCreate, project.Hook{Cmd: command})
	}
	return params
}

// unlocked is a meta store whose ordinals' lock is never held.
type unlocked struct{ provision.Meta }

func (unlocked) LockOrdinals(_ context.Context, do func() error) (bool, error) {
	return false, do()
}

// unreadable is an env store whose files cannot be read.
type unreadable struct{ provision.EnvFiles }

func (unreadable) Read(string) ([]dotenv.Line, error) {
	return nil, errors.New("read .env: permission denied")
}

// upstream is a repository whose branch existing is behind its origin.
type upstream struct {
	main    string
	linked  string
	service provision.Service
}

type upstreamSeed struct {
	Linked bool
	Dirty  bool
}

// behindOrigin builds upstream, existing checked out in a linked worktree (dirty
// or not) when seed asks.
func behindOrigin(t *testing.T, seed upstreamSeed) upstream {
	t.Helper()
	root := gittest.TempDir(t)
	params := gittest.FakeParams{
		Main:   filepath.Join(root, "main"),
		Branch: "main",
		Local:  []string{existing},
		Remote: []string{existing},
		Origin: map[string][]string{existing: {"upstream " + existing}},
	}
	linked := ""
	if seed.Linked {
		linked = filepath.Join(root, existing)
		params.Worktrees = []worktree.Worktree{{Path: linked, State: worktree.OnBranch{Branch: existing}}}
	}
	if seed.Dirty {
		params.Dirty = []string{linked}
	}
	return upstream{main: params.Main, linked: linked, service: provision.Service{Git: gittest.Build(t, params)}}
}
