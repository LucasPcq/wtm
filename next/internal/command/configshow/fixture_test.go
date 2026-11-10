package configshow_test

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// acme is config.toml as init writes it for acme: two .env files from their template, two create hooks, one clean hook, the parent's .env remembered.
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
on_clean = ["docker compose down"]

[wizard.remembered]
env_strategy = "parent"
`

// branches are acme's: main and develop, and origin's main.
type branches struct{}

func (branches) Branches(context.Context) (worktree.Branches, error) {
	return worktree.Branches{Local: []worktree.Branch{{Name: "main"}, {Name: "develop"}}, Remote: []worktree.Branch{{Name: "main"}}}, nil
}
