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

func refuseNewer(path string, data []byte) error {
	var file struct {
		Schema int `toml:"schema"`
	}
	if _, err := toml.Decode(string(data), &file); err != nil {
		return nil
	}
	return statefile.Refusal(statefile.Schema{Path: path, Found: file.Schema, Supported: projectSchema})
}

func writeSchema(ctx context.Context, stateDir string) error {
	_, err := statefile.Update(ctx, statefile.UpdateParams{
		Path:   filepath.Join(stateDir, schemasDirName, schemaFileName),
		Mutate: func([]byte) (statefile.Change, error) { return statefile.Write{Data: projectSchemaJSON}, nil },
	})
	return err
}
