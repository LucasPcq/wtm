package settings

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

// detected is what an acme repository suggests: two .env files with their template, its install and its teardown, and its folders.
func detected() project.Detected {
	return project.Detected{
		EnvFiles: []project.EnvFile{{Target: "apps/api/.env", Template: "apps/api/.env.example"}, {Target: "apps/web/.env", Template: "apps/web/.env.example"}},
		OnCreate: []project.FoundHook{{Hook: project.Hook{Cmd: "pnpm install"}, File: "pnpm-lock.yaml"}},
		OnClean:  []project.FoundHook{{Hook: project.Hook{Cmd: "docker compose down"}, File: "docker-compose.yml"}},
		Dirs:     []string{"apps", "apps/api", "apps/web"},
	}
}

// saved is config.toml as an earlier init wrote it, the web's .env file since removed by the user, a hook added in a folder and an answer remembered.
func saved() *project.Saved {
	return &project.Saved{
		Config: project.Config{
			Worktrees:  project.Worktrees{BasePath: "../acme.trees", BaseBranch: "main"},
			Env:        project.Env{Strategy: project.EnvMain, Files: []project.EnvFile{{Target: "apps/api/.env", Template: "apps/api/.env.example"}}},
			OnCreate:   []project.Hook{{Cmd: "pnpm install"}, {Cmd: "pnpm db:generate", Cwd: "packages/db"}},
			OnClean:    []project.Hook{},
			Remembered: map[string]string{project.RememberIsolation: "verbatim"},
		},
		Digest: "abc",
	}
}

// facts are the repository's facts, with config.toml as s.
func facts(s *project.Saved) Facts {
	return Facts{Saved: s, Detected: detected(), Branches: []string{"main", "develop"}, Head: "main"}
}

// store is a config.toml in memory, nil when there is none.
type store struct {
	saved *project.Saved
}

func (s *store) Saved() (project.Saved, error) {
	if s.saved == nil {
		return project.Saved{}, project.ErrConfigNotFound
	}
	return *s.saved, nil
}

func (s *store) Save(_ context.Context, rewrite project.Rewrite) (domain.Written, error) {
	s.saved = &project.Saved{Config: rewrite.Config, Digest: "written"}
	return domain.Written{Changed: true}, nil
}
