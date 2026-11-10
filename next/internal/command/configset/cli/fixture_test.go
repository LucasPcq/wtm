package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/config"
	"github.com/LucasPcq/wtm/next/internal/command/configset"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
	"github.com/LucasPcq/wtm/next/internal/service/settings"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"

	"github.com/stretchr/testify/require"
)

// acme is config.toml as init writes it for acme.
const acme = `schema = 1

[worktrees]
base_path = "../.trees"
base_branch = "main"

[env]
strategy = "example"

[[env.file]]
target = "apps/api/.env"
template = "apps/api/.env.example"

[[env.file]]
target = "apps/web/.env"
template = "apps/web/.env.example"

[hooks]
on_create = ["pnpm install", { cmd = "pnpm db:generate", cwd = "packages/db" }]
on_clean = [{ cmd = "docker compose down", continue_on_error = true }]

[wizard.remembered]
env_strategy = "parent"
`

// A world is a repository whose config.toml a test reads, none when Config is "".
type world struct {
	Config string
}

func (w world) env() uicli.Env {
	return uicli.Env{Cwd: "/acme", ProjectDir: "/acme"}
}

// opener is config show over the state directory dir, holding w's config.toml.
func (w world) opener(t *testing.T, dir string) func(context.Context, string) (configset.UseCase, error) {
	if w.Config != "" {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "config.toml"), []byte(w.Config), 0o644))
	}
	return func(context.Context, string) (configset.UseCase, error) {
		locks := &operationtest.Locks{}
		ops := operation.Service{Locker: locks, Publisher: locks, NewOpID: func() string { return "op-1" }, Now: time.Now}
		return configset.UseCase{Settings: settings.Service{Store: config.Project{StateDir: dir}}, Git: branches{}, Detect: func() project.Detected { return project.Detected{Dirs: []string{"packages/db"}} }, Operations: ops, Init: &domain.FollowUp{Command: domain.CommandInit}}, nil
	}
}

// branches are acme's: main and develop.
type branches struct{}

func (branches) Branches(context.Context) (worktree.Branches, error) {
	return worktree.Branches{Local: []worktree.Branch{{Name: "main"}, {Name: "develop"}}}, nil
}
