package store

import (
	"bytes"
	"context"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra/fs"
	"github.com/LucasPcq/wtm/internal/rules"
)

// ProjectConfig is config.toml, changed as it is on disk under its lock.
type ProjectConfig interface {
	Remember(ctx context.Context, change rules.RememberedChange) (Written, error)
}

// ConfigDisk is <StateDir>/config.toml.
type ConfigDisk struct {
	StateDir string
}

var _ ProjectConfig = ConfigDisk{}

// Remember changes the remembered answers alone; a change that keeps and
// forgets nothing writes nothing.
func (c ConfigDisk) Remember(ctx context.Context, change rules.RememberedChange) (Written, error) {
	if len(change.Remember) == 0 && len(change.Forget) == 0 {
		return Written{}, nil
	}
	return c.Update(ctx, func(project *domain.ProjectConfig) {
		project.Wizard.Remembered = rules.ApplyRemembered(project.Wizard.Remembered, change)
	})
}

// Update changes config.toml as it is on disk at the moment of the write, never
// the config a run loaded with its defaults applied: no default is written in,
// and no change another process made since is lost.
func (c ConfigDisk) Update(ctx context.Context, update func(*domain.ProjectConfig)) (Written, error) {
	path := filepath.Join(c.StateDir, domain.ConfigFileName)
	changed := false
	lock, err := fs.UpdateFile(ctx, fs.UpdateFileParams{
		Path: path,
		Mutate: func(current []byte) (fs.Change, error) {
			data, err := config.ReviseProject(config.ReviseProjectParams{Path: path, Current: current, Update: update})
			if err != nil {
				return nil, err
			}
			if bytes.Equal(current, data) {
				return fs.Keep{}, nil
			}
			changed = true
			return fs.Write{Data: data}, nil
		},
	})
	if err != nil {
		return Written{}, err
	}
	written := Written{Changed: changed, Unlocked: !lock.Held}
	return written, config.WriteProjectSchema(ctx, c.StateDir)
}
