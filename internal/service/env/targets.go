package env

import (
	"context"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
)

type AddEnvTargetsParams struct {
	StateDir string
	Targets  []domain.EnvFile
}

// AddEnvTargets appends provisioning targets to config.toml as it is on disk.
func AddEnvTargets(ctx context.Context, params AddEnvTargetsParams) error {
	return config.UpdateProjectConfig(ctx, config.UpdateProjectConfigParams{
		StateDir: params.StateDir,
		Update: func(project *domain.ProjectConfig) {
			project.Env.Files = append(project.Env.Files, params.Targets...)
		},
	})
}
