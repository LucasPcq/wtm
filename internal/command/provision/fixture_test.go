package provision_test

import (
	"maps"
	"path/filepath"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/command/provision"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra/git"
	"github.com/LucasPcq/wtm/internal/store"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
	"github.com/LucasPcq/wtm/internal/testutil/storetest"
)

const (
	branch   = "feat/x"
	existing = "feature"
	mainEnv  = "PORT=3000\n"
)

var createdAt = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

type world struct {
	root     string
	main     string
	stateDir string
	git      git.ReadWriter
	meta     store.Meta
	ports    provision.Ports
}

func onDisk(t *testing.T) world {
	t.Helper()
	root := gittest.TempDir(t)
	main := filepath.Join(root, "main")
	repo := gittest.Build(t, gittest.FakeParams{Main: main, Branch: "main", Local: []string{existing}})
	stateDir := filepath.Join(main, ".git", "wtm")
	disk := storetest.OnDisk(stateDir)
	writeFile(t, filepath.Join(main, ".env"), mainEnv)
	return world{
		root:     root,
		main:     main,
		stateDir: stateDir,
		git:      repo,
		meta:     disk.Meta,
		ports:    provision.Ports{Git: repo, Meta: disk.Meta, Env: disk.Env, StateDir: stateDir, Now: func() time.Time { return createdAt }},
	}
}

type seeds struct {
	GitFail map[string]error
	Store   storetest.FakeParams
}

func inMemory(t *testing.T, seed seeds) (world, *storetest.Fake) {
	t.Helper()
	root := gittest.TempDir(t)
	main := filepath.Join(root, "main")
	repo := gittest.NewFake(gittest.FakeParams{Main: main, Branch: "main", Local: []string{existing}, Fail: seed.GitFail})
	seeded := seed.Store
	seeded.Files = maps.Clone(seeded.Files)
	if seeded.Files == nil {
		seeded.Files = map[string]string{}
	}
	seeded.Files[filepath.Join(main, ".env")] = mainEnv
	files := storetest.NewFake(seeded)
	stateDir := filepath.Join(main, ".git", "wtm")
	return world{
		root:     root,
		main:     main,
		stateDir: stateDir,
		git:      repo,
		meta:     files,
		ports:    provision.Ports{Git: repo, Meta: files, Env: files, StateDir: stateDir, Now: func() time.Time { return createdAt }},
	}, files
}

func (w world) params() provision.UnitParams {
	return provision.UnitParams{
		Ports:   w.ports,
		Project: provision.Project{MainPath: w.main, EnvFiles: []domain.EnvFile{{Target: ".env"}}},
		Worktree: provision.Worktree{
			Branch:   branch,
			Path:     w.path(branch),
			Checkout: git.NewBranch{Name: branch, From: git.LocalBranch{Name: "main"}},
			Source:   "main",
			Strategy: domain.EnvStrategyMain,
		},
	}
}

func (w world) existing() provision.UnitParams {
	params := w.params()
	params.Worktree.Branch = existing
	params.Worktree.Path = w.path(existing)
	params.Worktree.Checkout = git.ExistingBranch{Name: existing}
	params.Worktree.Origin = provision.Origin{ExistingBranch: true}
	return params
}

func (w world) path(name string) string {
	return filepath.Join(w.root, filepath.Base(name))
}

func portLinked(params provision.UnitParams) provision.UnitParams {
	params.Project.Run = domain.RunConfig{
		Jobs:     []domain.JobConfig{{Name: "web", Kind: domain.JobKindService, Cmd: "npm start", Ports: map[string]int{"http": 3000}}},
		EnvPorts: []domain.EnvPortLink{{File: ".env", Key: "PORT", Job: "web", Port: "http"}},
	}
	return params
}

func composed(params provision.UnitParams) provision.UnitParams {
	params.Project.Run = domain.RunConfig{
		Jobs: []domain.JobConfig{{Name: "db", Kind: domain.JobKindService, Cmd: "docker compose up -d", Ports: map[string]int{"pg": 5432}}},
	}
	return params
}

func hooked(params provision.UnitParams, commands ...string) provision.UnitParams {
	for _, command := range commands {
		params.Project.OnCreate = append(params.Project.OnCreate, domain.HookCommand{Cmd: command})
	}
	return params
}
