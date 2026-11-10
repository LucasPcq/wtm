package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/config"
	"github.com/LucasPcq/wtm/next/internal/command/configshow"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
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
func (w world) opener(t *testing.T, dir string) func(context.Context, string) (configshow.UseCase, error) {
	if w.Config != "" {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "config.toml"), []byte(w.Config), 0o644))
	}
	return func(context.Context, string) (configshow.UseCase, error) {
		return configshow.UseCase{Settings: settings.Service{Store: config.Project{StateDir: dir}}, Git: branches{}, Init: &domain.FollowUp{Command: domain.CommandInit}, Edit: &domain.FollowUp{Command: "config-edit", Code: "config-show.edit", Message: "fix it in your editor"}}, nil
	}
}

// branches are acme's: main and develop.
type branches struct{}

func (branches) Branches(context.Context) (worktree.Branches, error) {
	return worktree.Branches{Local: []worktree.Branch{{Name: "main"}, {Name: "develop"}}}, nil
}
