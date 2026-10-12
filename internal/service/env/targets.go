package env

import (
	"context"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/store"
)

type AddEnvTargetsParams struct {
	StateDir string
	Targets  []domain.EnvFile
}

// AddEnvTargets appends provisioning targets to config.toml as it is on disk.
func AddEnvTargets(ctx context.Context, params AddEnvTargetsParams) error {
	return store.ConfigDisk{StateDir: params.StateDir}.Update(ctx, func(project *domain.ProjectConfig) {
		project.Env.Files = append(project.Env.Files, params.Targets...)
	})
}
