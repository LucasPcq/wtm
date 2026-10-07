// Package memory keeps the answers a repository asked the wizard never to ask again.
package memory

import (
	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/rules"
)

type RememberParams struct {
	StateDir string
	Change   rules.RememberedChange
}

// Remember rewrites config.toml from what is on disk, not from the config the run
// loaded with its defaults applied, which would write them in.
func Remember(params RememberParams) error {
	if len(params.Change.Remember) == 0 && len(params.Change.Forget) == 0 {
		return nil
	}
	project, err := config.LoadProjectRaw(params.StateDir)
	if err != nil {
		return err
	}
	project.Wizard.Remembered = rules.ApplyRemembered(project.Wizard.Remembered, params.Change)
	return config.WriteProjectConfig(config.WriteProjectConfigParams{StateDir: params.StateDir, Config: project})
}
