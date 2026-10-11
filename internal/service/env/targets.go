package env

import (
	"context"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
)

type AddEnvTargetsParams struct {
	StateDir string
	Project  domain.ProjectConfig
	Targets  []domain.EnvFile
}

// AddEnvTargets appends provisioning targets to config.toml. Project is the
// config as loaded: the file is re-rendered whole from it.
func AddEnvTargets(ctx context.Context, params AddEnvTargetsParams) error {
	project := params.Project
	project.Env.Files = append(project.Env.Files, params.Targets...)
	return config.WriteProjectConfig(ctx, config.WriteProjectConfigParams{StateDir: params.StateDir, Config: project})
}
