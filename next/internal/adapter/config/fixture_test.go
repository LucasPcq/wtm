package config

import (
	"context"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/configtest"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

const fullConfig = `
[worktrees]
base_path = ".trees"
base_branch = "develop"

[env]
strategy = "main"

[[env.file]]
target = ".env"

[[env.file]]
target = "apps/api/.env"

[hooks]
on_create = [
  "pnpm install",
  { cmd = "pnpm install", cwd = "apps/api" },
]
`

const minimalConfig = `
[worktrees]
`

const newerSchema = "schema = 99\n"

// awkward holds every character a TOML string must escape, and awkwardConfig
// writes it in every value config.toml holds, to prove a rewrite round-trips.
const awkward = "say \"hi\" \\ C:\\tmp 'q'\nnext\ttab \x7f"

const awkwardConfig = `
[worktrees]
base_path = "../say \"hi\" \\ C:\\tmp 'q'\nnext\ttab \u007f"
base_branch = "feat/say \"hi\" \\ C:\\tmp 'q'\nnext\ttab \u007f"

[env]
strategy = "example"

[[env.file]]
target = "say \"hi\" \\ C:\\tmp 'q'\nnext\ttab \u007f/.env"
template = "say \"hi\" \\ C:\\tmp 'q'\nnext\ttab \u007f/.env.example"

[hooks]
on_create = [
  "say \"hi\" \\ C:\\tmp 'q'\nnext\ttab \u007f",
  { cmd = "say \"hi\" \\ C:\\tmp 'q'\nnext\ttab \u007f", cwd = "say \"hi\" \\ C:\\tmp 'q'\nnext\ttab \u007f", continue_on_error = true },
]
on_clean = [
  "say \"hi\" \\ C:\\tmp 'q'\nnext\ttab \u007f",
]
`

type sample struct {
	Name  string         `toml:"name"`
	Port  int            `toml:"port"`
	Hooks map[string]any `toml:"hooks"`
}

// port is what both Project and configtest.Fake implement, so one test runs
// against each.
type port interface {
	Load() (project.Config, project.Global, error)
	LoadRun() (project.Run, error)
	Remember(ctx context.Context, answers map[string]string) (domain.Written, error)
}

type implementation struct {
	name string
	port port
}

// A seed is a project's state directory: its config.toml as Load returns it,
// nil for none, whether it does not parse, and the schema it carries.
type seed struct {
	Config     *project.Config
	Unreadable bool
	Schema     int
}

// implementations returns the project on disk and the fake, both seeded with s.
func implementations(t *testing.T, s seed) []implementation {
	t.Helper()
	return []implementation{
		{name: "file", port: onDisk(t, s)},
		{name: "fake", port: configtest.NewFake(configtest.FakeParams{Config: s.Config, Unreadable: s.Unreadable, Schema: s.Schema})},
	}
}

// seeded is a config.toml with every default written out and an answer remembered.
func seeded() *project.Config {
	return &project.Config{
		Worktrees:  project.Worktrees{BasePath: ".trees", BaseBranch: "develop"},
		Env:        project.Env{Strategy: project.EnvMain, Files: []project.EnvFile{{Target: ".env"}}},
		OnCreate:   []project.Hook{},
		Remembered: map[string]string{project.RememberEnvStrategy: "main"},
	}
}
