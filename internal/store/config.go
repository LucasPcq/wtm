package store

import (
	"context"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra/fs"
	"github.com/LucasPcq/wtm/internal/rules"
)

// ConfigDisk is <StateDir>/config.toml.
type ConfigDisk struct {
	StateDir string
}

var _ ProjectConfig = ConfigDisk{}

// Remember changes the remembered answers alone; a change that keeps and
// forgets nothing writes nothing.
func (c ConfigDisk) Remember(ctx context.Context, change rules.RememberedChange) error {
	if len(change.Remember) == 0 && len(change.Forget) == 0 {
		return nil
	}
	return c.Update(ctx, func(project *domain.ProjectConfig) {
		project.Wizard.Remembered = rules.ApplyRemembered(project.Wizard.Remembered, change)
	})
}

// Update changes config.toml as it is on disk at the moment of the write, never
// the config a run loaded with its defaults applied: no default is written in,
// and no change another process made since is lost.
func (c ConfigDisk) Update(ctx context.Context, update func(*domain.ProjectConfig)) error {
	path := filepath.Join(c.StateDir, domain.ConfigFileName)
	_, err := fs.UpdateFile(ctx, fs.UpdateFileParams{
		Path: path,
		Mutate: func(current []byte) (fs.Change, error) {
			data, err := config.ReviseProject(config.ReviseProjectParams{Path: path, Current: current, Update: update})
			if err != nil {
				return nil, err
			}
			return fs.Write{Data: data}, nil
		},
	})
	if err != nil {
		return err
	}
	return config.WriteProjectSchema(ctx, c.StateDir)
}
