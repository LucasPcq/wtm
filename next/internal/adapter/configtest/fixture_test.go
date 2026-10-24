package configtest

import "github.com/LucasPcq/wtm/next/internal/domain/project"

// seeded is a config.toml with a .env file, a hook and an answer remembered.
func seeded() *project.Config {
	return &project.Config{
		Worktrees:  project.Worktrees{BasePath: ".trees", BaseBranch: "main"},
		Env:        project.Env{Strategy: project.EnvMain, Files: []project.EnvFile{{Target: ".env"}}},
		OnCreate:   []project.Hook{{Cmd: "make setup"}},
		Remembered: map[string]string{project.RememberEnvStrategy: "main"},
	}
}
