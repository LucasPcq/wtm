package config

import (
	"context"
	_ "embed"
	"maps"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/LucasPcq/wtm/next/internal/adapter/statefile"
	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/project"
)

const (
	schemasDirName = "schemas"
	schemaFileName = "project.schema.json"
)

//go:embed project.schema.json
var projectSchemaJSON []byte

type RememberParams struct {
	StateDir string
	Answers  map[string]string
}

func Remember(ctx context.Context, params RememberParams) (wtm.Written, error) {
	if len(params.Answers) == 0 {
		return wtm.Written{}, nil
	}
	path := filepath.Join(params.StateDir, configFileName)
	written, err := statefile.Update(ctx, statefile.UpdateParams{
		Path: path,
		Mutate: func(current []byte) (statefile.Change, error) {
			next, err := remembered(rememberedParams{Path: path, Current: current, Answers: params.Answers})
			if err != nil {
				return nil, err
			}
			return statefile.Write{Data: next}, nil
		},
	})
	if err != nil {
		return wtm.Written{}, err
	}
	return written, writeSchema(ctx, params.StateDir)
}

type rememberedParams struct {
	Path    string
	Current []byte
	Answers map[string]string
}

func remembered(params rememberedParams) ([]byte, error) {
	if params.Current == nil {
		return nil, project.ErrConfigNotFound
	}
	if err := refuseNewer(parseParams{Path: params.Path, Data: params.Current}); err != nil {
		return nil, err
	}
	saved, err := parseProject(parseParams{Path: params.Path, Data: params.Current})
	if err != nil {
		return nil, err
	}
	answers := map[string]string{}
	maps.Copy(answers, saved.Config.Remembered)
	maps.Copy(answers, params.Answers)
	saved.Config.Remembered = answers
	return render(saved)
}

func refuseNewer(params parseParams) error {
	var file struct {
		Schema int `toml:"schema"`
	}
	if _, err := toml.Decode(string(params.Data), &file); err != nil {
		return nil
	}
	return statefile.Refusal(statefile.Schema{Path: params.Path, Found: file.Schema, Supported: projectSchema})
}

func writeSchema(ctx context.Context, stateDir string) error {
	_, err := statefile.Update(ctx, statefile.UpdateParams{
		Path:   filepath.Join(stateDir, schemasDirName, schemaFileName),
		Mutate: func([]byte) (statefile.Change, error) { return statefile.Write{Data: projectSchemaJSON}, nil },
	})
	return err
}
