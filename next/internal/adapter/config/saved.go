package config

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/LucasPcq/wtm/next/internal/adapter/statefile"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

// Saved returns the project's config.toml as written, before any default applies, with the digest of its bytes. A missing config.toml is project.ErrConfigNotFound, wrapped; one that does not parse or validate a *project.InvalidConfigError.
func (p Project) Saved() (project.Saved, error) {
	path := filepath.Join(p.StateDir, configFileName)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return project.Saved{}, fmt.Errorf("project config: %w", project.ErrConfigNotFound)
	}
	if err != nil {
		return project.Saved{}, fmt.Errorf("project config: %w", err)
	}

	saved, err := parseProject(path, data)
	if err == nil {
		err = validateProject(withDefaults(saved.Config))
	}
	if err != nil {
		return project.Saved{}, &project.InvalidConfigError{Reason: err}
	}

	return project.Saved{Config: saved.Config, Digest: digest(data)}, nil
}

// Save writes config.toml rendered whole from rewrite.Config, comments included, then the JSON schema beside it, and returns what it wrote. A file whose bytes are no longer those rewrite replaces, created or removed since included, is refused with project.ErrConfigChanged, and one a newer wtm wrote with the statefile's schema error.
func (p Project) Save(ctx context.Context, rewrite project.Rewrite) (domain.Written, error) {
	path := filepath.Join(p.StateDir, configFileName)
	written, err := statefile.Update(ctx, statefile.UpdateParams{
		Path: path,
		Mutate: func(current []byte) (statefile.Change, error) {
			if digest(current) != rewrite.Replaces {
				return nil, project.ErrConfigChanged
			}
			if err := refuseNewer(path, current); err != nil {
				return nil, err
			}
			data, err := render(savedProject{Config: rewrite.Config})
			if err != nil {
				return nil, err
			}
			return statefile.Write{Data: data}, nil
		},
	})
	if err != nil {
		return domain.Written{}, err
	}

	return written, writeSchema(ctx, p.StateDir)
}

// digest is the digest of a file's bytes, "" for no file.
func digest(data []byte) string {
	if data == nil {
		return ""
	}

	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
