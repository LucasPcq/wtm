package initialize_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/config"
	"github.com/LucasPcq/wtm/next/internal/adapter/detect"
	"github.com/LucasPcq/wtm/next/internal/adapter/git"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/oplock"
	"github.com/LucasPcq/wtm/next/internal/command/initialize"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
	"github.com/LucasPcq/wtm/next/internal/service/settings"
)

var startedAt = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)

type world struct {
	root       string
	main       string
	stateDir   string
	repo       git.Repo
	registered *int
	usecase    initialize.UseCase
}

// newWorld is a repository wtm has not set up: main, a worktree made elsewhere, two .env files with their template, a pnpm lockfile and a docker compose file.
func newWorld(t *testing.T) world {
	t.Helper()
	root := gittest.TempDir(t)
	main := filepath.Join(root, "main")
	repo := gittest.Build(t, gittest.FakeParams{
		Main:      main,
		Branch:    "main",
		Local:     []string{"develop"},
		Remote:    []string{"main"},
		Worktrees: []worktree.Worktree{{Path: filepath.Join(root, "elsewhere"), State: worktree.OnBranch{Branch: "feat/old"}}},
	})
	for name, content := range map[string]string{
		"apps/api/.env.example": "PORT=3000\n",
		"apps/web/.env.example": "PORT=4000\n",
		"pnpm-lock.yaml":        "lockfileVersion: 9\n",
		"docker-compose.yml":    "services: {}\n",
	} {
		writeFile(t, filepath.Join(main, name), content)
	}

	stateDir := filepath.Join(main, ".git", "wtm")
	project := config.Project{StateDir: stateDir, GlobalPath: filepath.Join(root, "no-global.toml")}
	registered := new(int)
	ids := 0
	w := world{root: root, main: main, stateDir: stateDir, repo: repo, registered: registered}
	w.usecase = initialize.UseCase{
		Git:       repo,
		Detect:    detect.Repository{Dir: main}.Detect,
		RunConfig: project,
		Register:  func(context.Context) { *registered++ },
		Settings:  settings.Service{Store: project},
		Operations: operation.Service{
			Locker:    locks{oplock.Locks{StateDir: stateDir}},
			Publisher: &operationtest.Recorder{},
			NewOpID: func() string {
				ids++
				return "op-" + string(rune('0'+ids))
			},
			Now: func() time.Time { return startedAt },
		},
		MainPath:   main,
		ConfigPath: filepath.Join(stateDir, "config.toml"),
	}
	return w
}

type locks struct {
	oplock.Locks
}

func (l locks) Take(op domain.Operation) (operation.Hold, error) {
	return l.Locks.Take(op)
}

// reInit is config.toml as a first init wrote it, the web's .env file since removed by hand and a hook in a folder added.
const reInit = `[worktrees]
base_path = "../.trees"
base_branch = "main"

[env]
strategy = "example"

[[env.file]]
target = "apps/api/.env"
template = "apps/api/.env.example"

[hooks]
on_create = [
  "pnpm install",
  { cmd = "pnpm db:generate", cwd = "apps/api" },
]
on_clean = []

[wizard.remembered]
isolation = "verbatim"
`
