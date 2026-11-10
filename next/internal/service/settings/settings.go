// Package settings is config.toml as a document: its settings as the fields of a form, filled from the file as written and from what the repository suggests, and the file written back whole. init asks them, and a read of the settings or a change to one shares them.
package settings

import (
	"context"
	"errors"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

// Service reads and writes config.toml through its Store.
type Service struct {
	Store Store
}

// Read is config.toml as written, nil when there is none; one that does not parse or validate is a *project.InvalidConfigError.
func (s Service) Read() (*project.Saved, error) {
	saved, err := s.Store.Saved()
	if errors.Is(err, project.ErrConfigNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &saved, nil
}

// Write writes config.toml whole, as rewrite says, and returns what it wrote; see Store.Save for its refusals.
func (s Service) Write(ctx context.Context, rewrite project.Rewrite) (domain.Written, error) {
	return s.Store.Save(ctx, rewrite)
}
