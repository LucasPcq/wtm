package config

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/LucasPcq/wtm/next/internal/adapter/statefile"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

// Saved returns the project's config.toml as written, before any default applies and with its values unchecked, the digest of its bytes and its schema. A missing config.toml is project.ErrConfigNotFound, wrapped; one that does not parse a *project.InvalidConfigError saying where it stopped. A file a newer wtm wrote is read leniently, its keys this wtm does not know left aside.
func (p Project) Saved() (project.Saved, error) {
	path := filepath.Join(p.StateDir, configFileName)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return project.Saved{}, fmt.Errorf("project config: %w", project.ErrConfigNotFound)
	}
	if err != nil {
		return project.Saved{}, fmt.Errorf("project config: %w", err)
	}

	schema := schemaOf(data)
	parse := parseProject
	if schema > projectSchema {
		parse = parseNewer
	}
	saved, err := parse(path, data)
	if err != nil {
		return project.Saved{}, invalidConfig(err)
	}

	return project.Saved{Config: saved.Config, Digest: digest(data), Schema: schema}, nil
}

// schemaOf is the schema config.toml's bytes carry, 0 when they name none or do not parse.
func schemaOf(data []byte) int {
	var file struct {
		Schema int `toml:"schema"`
	}
	_, _ = toml.Decode(string(data), &file)
	return file.Schema
}

// parseNewer decodes a config.toml a newer wtm wrote, its unknown keys left aside.
func parseNewer(path string, data []byte) (savedProject, error) {
	var file projectFile
	if _, err := toml.Decode(string(data), &file); err != nil {
		return savedProject{}, fmt.Errorf("parse %s: %w", path, err)
	}

	return savedOf(file)
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
			if err := refuseNewer(path, current, projectSchema); err != nil {
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
