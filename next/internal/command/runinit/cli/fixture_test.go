package cli

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/compose"
	"github.com/LucasPcq/wtm/next/internal/adapter/config"
	"github.com/LucasPcq/wtm/next/internal/adapter/detect"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/store"
	"github.com/LucasPcq/wtm/next/internal/command/runinit"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/stretchr/testify/require"
)

type session = form.Session[runinit.Request, runinit.Questions, runinit.Outcome]

var startedAt = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)

// acme is the monorepo of the ticket: pnpm workspaces and a turbo root dev script, apps api, web, admin (its port a literal of its script) and worker, packages/db with migrate and seed, docker-compose.yml with postgres, redis and mailpit (8025 and 1025), and five .env files, apps/web's a .env.local config.toml does not copy.
var acme = map[string]string{
	"package.json":             `{"name": "acme", "packageManager": "pnpm@9.12.0", "scripts": {"dev": "turbo run dev", "build": "turbo run build"}}`,
	"pnpm-workspace.yaml":      "packages:\n  - \"apps/*\"\n  - \"packages/*\"\n",
	"apps/api/package.json":    `{"name": "@acme/api", "scripts": {"dev": "tsx watch src/server.ts", "build": "tsc"}}`,
	"apps/web/package.json":    `{"name": "@acme/web", "scripts": {"dev": "next dev", "build": "next build"}}`,
	"apps/admin/package.json":  `{"name": "@acme/admin", "scripts": {"dev": "next dev -p 3001", "build": "next build"}}`,
	"apps/worker/package.json": `{"name": "@acme/worker", "scripts": {"start": "tsx watch src/worker.ts"}}`,
	"packages/db/package.json": `{"name": "@acme/db", "scripts": {"migrate": "prisma migrate deploy", "seed": "tsx seed.ts"}}`,
	"docker-compose.yml":       "services:\n  postgres:\n    image: postgres:16\n    ports:\n      - \"5432:5432\"\n  redis:\n    image: redis:7\n    container_name: acme-redis\n    ports:\n      - \"6379:6379\"\n  mailpit:\n    image: axllent/mailpit\n    ports:\n      - \"8025:8025\"\n      - \"1025:1025\"\n",
	"apps/api/.env.example":    "PORT=4000\nDATABASE_URL=postgres://postgres:postgres@localhost:5432/acme\nREDIS_URL=redis://localhost:6379\n",
	"apps/api/.env":            "PORT=4000\nDATABASE_URL=postgres://postgres:postgres@localhost:5432/acme\nREDIS_URL=redis://localhost:6379\n",
	"apps/web/.env.local":      "PORT=3000\nNEXT_PUBLIC_API_URL=http://localhost:4000\n",
	"apps/admin/.env.example":  "NEXT_PUBLIC_API_URL=http://localhost:4000\n",
	"apps/worker/.env.example": "REDIS_URL=redis://localhost:6379\nSMTP_PORT=1025\n",
	"packages/db/.env.example": "DATABASE_URL=postgres://postgres:postgres@localhost:5432/acme\n",
}

// acmeConfig is config.toml as wtm init writes it for acme, apps/web/.env.local left out.
const acmeConfig = `[worktrees]
base_path = "../acme.trees"
base_branch = "main"

[env]
strategy = "example"

[[env.file]]
target = "apps/admin/.env"
template = "apps/admin/.env.example"

[[env.file]]
target = "apps/api/.env"
template = "apps/api/.env.example"

[[env.file]]
target = "apps/worker/.env"
template = "apps/worker/.env.example"

[[env.file]]
target = "packages/db/.env"
template = "packages/db/.env.example"
`

// world is acme on disk, its main worktree and feat/billing, worktree 2.
type world struct {
	main     string
	stateDir string
	locks    *operationtest.Locks
	repo     *gittest.Fake
}

func newWorld(t *testing.T) world {
	t.Helper()
	main := filepath.Join(gittest.TempDir(t), "acme")
	for name, content := range acme {
		writeFile(t, filepath.Join(main, filepath.FromSlash(name)), content)
	}
	w := world{main: main, stateDir: filepath.Join(main, ".git", "wtm"), locks: &operationtest.Locks{}}
	writeFile(t, filepath.Join(w.stateDir, "config.toml"), acmeConfig)
	w.repo = gittest.NewFake(gittest.FakeParams{Main: main, Branch: "main", Worktrees: []worktree.Worktree{{Path: filepath.Join(filepath.Dir(main), "acme.trees", "feat-billing"), State: worktree.OnBranch{Branch: billing}}}})
	_, err := store.Meta{StateDir: w.stateDir}.Update(t.Context(), worktree.MetaUpdate{Branch: billing, Change: func(stored *worktree.Meta) error {
		stored.Ordinal, stored.SourceBranch = 2, "main"
		return nil
	}})
	require.NoError(t, err)
	return w
}

func (w world) env() uicli.Env {
	return uicli.Env{Cwd: w.main, ProjectDir: w.main}
}

func (w world) open(context.Context, string) (runinit.UseCase, error) {
	return w.usecase(), nil
}

func (w world) usecase() runinit.UseCase {
	project := config.Project{StateDir: w.stateDir, GlobalPath: filepath.Join(w.stateDir, "no-global.toml")}
	return runinit.UseCase{
		RunConfig:  project,
		Settings:   project,
		Compose:    compose.Repository{Dir: w.main},
		Detector:   detect.Repository{Dir: w.main},
		EnvFiles:   store.EnvFiles{},
		Meta:       store.Meta{StateDir: w.stateDir},
		Repository: repository.Service{Config: project, Git: w.repo, MainPath: w.main},
		Operations: operation.Service{
			Locker:    w.locks,
			Publisher: w.locks,
			NewOpID:   func() string { return "op-1" },
			Now:       func() time.Time { return startedAt },
		},
	}
}

const billing = "feat/billing"
