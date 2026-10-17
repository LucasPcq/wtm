package repository_test

import "github.com/LucasPcq/wtm/next/internal/domain/project"

// configured is a project's config as Observe loads it: its config.toml or the
// error loading it, and the error loading its run.toml.
type configured struct {
	config  project.Config
	err     error
	runErr  error
	loadRun *int
}

func (c configured) Load() (project.Config, project.Global, error) {
	return c.config, project.Global{}, c.err
}

func (c configured) LoadRun() (project.Run, error) {
	*c.loadRun++
	return project.Run{}, c.runErr
}
