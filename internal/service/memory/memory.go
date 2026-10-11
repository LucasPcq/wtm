// Package memory keeps the answers a repository asked the wizard never to ask again.
package memory

import (
	"context"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

type RememberParams struct {
	StateDir string
	Change   rules.RememberedChange
}

// Remember changes config.toml as it is on disk, not the config the run loaded
// with its defaults applied, which would write them in.
func Remember(ctx context.Context, params RememberParams) error {
	if len(params.Change.Remember) == 0 && len(params.Change.Forget) == 0 {
		return nil
	}
	return config.UpdateProjectConfig(ctx, config.UpdateProjectConfigParams{
		StateDir: params.StateDir,
		Update: func(project *domain.ProjectConfig) {
			project.Wizard.Remembered = rules.ApplyRemembered(project.Wizard.Remembered, params.Change)
		},
	})
}
