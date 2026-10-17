package config

import (
	"context"
	_ "embed"
	"maps"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/LucasPcq/wtm/next/internal/adapter/statefile"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

const (
	schemasDirName = "schemas"
	schemaFileName = "project.schema.json"
)

//go:embed project.schema.json
var projectSchemaJSON []byte

// Remember merges answers into the [wizard.remembered] table of the
// config.toml in stateDir, re-rendering the file, then writes the JSON schema
// beside it. It returns what it wrote; no answers write nothing. A missing
// config.toml is project.ErrConfigNotFound, and a config.toml written by a
// newer wtm is refused with the statefile's schema error.
func Remember(ctx context.Context, stateDir string, answers map[string]string) (domain.Written, error) {
	if len(answers) == 0 {
		return domain.Written{}, nil
	}

	path := filepath.Join(stateDir, configFileName)
	written, err := statefile.Update(ctx, statefile.UpdateParams{
		Path: path,
		Mutate: func(current []byte) (statefile.Change, error) {
			next, err := remembered(path, current, answers)
			if err != nil {
				return nil, err
			}
			return statefile.Write{Data: next}, nil
		},
	})
	if err != nil {
		return domain.Written{}, err
	}

	return written, writeSchema(ctx, stateDir)
}

// remembered returns the config.toml current with answers merged over the
// answers it already remembers, an answer given now winning.
func remembered(path string, current []byte, answers map[string]string) ([]byte, error) {
	if current == nil {
		return nil, project.ErrConfigNotFound
	}

	if err := refuseNewer(path, current); err != nil {
		return nil, err
	}

	saved, err := parseProject(path, current)
	if err != nil {
		return nil, err
	}

	merged := map[string]string{}
	maps.Copy(merged, saved.Config.Remembered)
	maps.Copy(merged, answers)
	saved.Config.Remembered = merged
	return render(saved)
}

// refuseNewer returns the statefile's refusal when data carries a schema newer
// than this wtm writes.
func refuseNewer(path string, data []byte) error {
	var file struct {
		Schema int `toml:"schema"`
	}
	if _, err := toml.Decode(string(data), &file); err != nil {
		// parseProject reports a malformed file with its position.
		return nil
	}

	return statefile.Refusal(statefile.Schema{Path: path, Found: file.Schema, Supported: projectSchema})
}

// writeSchema writes the JSON schema config.toml's first line points editors
// to, so it matches the version that wrote the file.
func writeSchema(ctx context.Context, stateDir string) error {
	_, err := statefile.Update(ctx, statefile.UpdateParams{
		Path:   filepath.Join(stateDir, schemasDirName, schemaFileName),
		Mutate: func([]byte) (statefile.Change, error) { return statefile.Write{Data: projectSchemaJSON}, nil },
	})
	return err
}
