package config

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/LucasPcq/wtm/internal/domain"
	domainstatefile "github.com/LucasPcq/wtm/internal/domain/statefile"
	"github.com/LucasPcq/wtm/internal/infra/fs"
	"github.com/LucasPcq/wtm/internal/rules/statefile"
	"github.com/LucasPcq/wtm/internal/schemas"
)

var ErrRunFileExists = errors.New("run file already exists")

// projectTemplateData is the unified view rendered by the config template.
// Both the init wizard answers and a full ProjectConfig (for re-init) convert
// to it, so the template emits every section's actual values.
type projectTemplateData struct {
	Schema      int
	BasePath    string
	BaseBranch  string
	SkipEnv     bool
	EnvStrategy string
	EnvFiles    []domain.EnvFile
	SkipHooks   bool
	OnCreate    []domain.HookCommand
	SkipClean   bool
	OnClean     []domain.HookCommand
	Remembered  map[string]string
}

type WriteProjectParams struct {
	StateDir string
	Answers  domain.InitProjectAnswers
}

func WriteProject(ctx context.Context, params WriteProjectParams) error {
	return renderProjectConfig(ctx, renderParams{StateDir: params.StateDir, Render: func([]byte) (projectTemplateData, error) {
		return answersToTemplate(params.Answers), nil
	}})
}

type WriteProjectConfigParams struct {
	StateDir string
	Config   domain.ProjectConfig
}

// WriteProjectConfig re-renders config.toml from a full ProjectConfig, preserving
// every section's current values (only manual comments are regenerated).
func WriteProjectConfig(ctx context.Context, params WriteProjectConfigParams) error {
	return renderProjectConfig(ctx, renderParams{StateDir: params.StateDir, Render: func([]byte) (projectTemplateData, error) {
		return configToTemplate(params.Config), nil
	}})
}

type UpdateProjectConfigParams struct {
	StateDir string
	Update   func(*domain.ProjectConfig)
}

// UpdateProjectConfig changes config.toml as it is on disk at the moment of the
// write, under its lock: no default is added, and no change another process
// made since this one loaded the config is lost.
func UpdateProjectConfig(ctx context.Context, params UpdateProjectConfigParams) error {
	path := filepath.Join(params.StateDir, domain.ConfigFileName)
	return renderProjectConfig(ctx, renderParams{StateDir: params.StateDir, Render: func(current []byte) (projectTemplateData, error) {
		if current == nil {
			return projectTemplateData{}, domain.ErrConfigNotFound
		}
		project, err := parseProjectConfig(path, current)
		if err != nil {
			return projectTemplateData{}, err
		}
		params.Update(&project)
		return configToTemplate(project), nil
	}})
}

type renderParams struct {
	StateDir string
	Render   func(current []byte) (projectTemplateData, error)
}

func renderProjectConfig(ctx context.Context, params renderParams) error {
	path := filepath.Join(params.StateDir, domain.ConfigFileName)
	_, err := fs.UpdateFile(ctx, fs.UpdateFileParams{
		Path: path,
		Mutate: func(current []byte) ([]byte, error) {
			if err := refuseNewer(refuseNewerParams{Path: path, Current: current, Supported: domainstatefile.ProjectConfigSchema}); err != nil {
				return nil, err
			}
			data, err := params.Render(current)
			if err != nil {
				return nil, err
			}
			data.Schema = statefile.Written(domainstatefile.ProjectConfigSchema)
			var buf bytes.Buffer
			if err := parsedTemplate.Execute(&buf, data); err != nil {
				return nil, fmt.Errorf("render template: %w", err)
			}
			return buf.Bytes(), nil
		},
	})
	if err != nil {
		return err
	}
	return writeSchema(ctx, writeSchemaParams{Dir: params.StateDir, Schema: schemas.Project})
}

type refuseNewerParams struct {
	Path      string
	Current   []byte
	Supported int
}

// refuseNewer stops a write over a file a newer wtm wrote. A file that does not
// parse has no schema to read and is overwritten, as it always was.
func refuseNewer(params refuseNewerParams) error {
	var file struct {
		Schema int `toml:"schema"`
	}
	if _, err := toml.Decode(string(params.Current), &file); err != nil {
		return nil
	}
	return statefile.Refusal(statefile.RefusalParams{
		Path:   params.Path,
		Schema: statefile.SchemaParams{Found: file.Schema, Supported: params.Supported},
	})
}

type writeSchemaParams struct {
	Dir    string
	Schema schemas.Schema
}

// writeSchema puts this binary's schema beside the file that points at it, on
// every write rather than once at init: an upgrade otherwise leaves the editor
// validating against the version the repository was first set up with. Loading
// writes nothing — `wtm schema dump` refreshes them without touching a config.
func writeSchema(ctx context.Context, params writeSchemaParams) error {
	_, err := fs.UpdateFile(ctx, fs.UpdateFileParams{
		Path:   filepath.Join(params.Dir, domain.SchemasDirName, params.Schema.Filename()),
		Mutate: func([]byte) ([]byte, error) { return params.Schema.Bytes(), nil },
	})
	return err
}

func answersToTemplate(a domain.InitProjectAnswers) projectTemplateData {
	return projectTemplateData{
		BasePath:    a.BasePath,
		BaseBranch:  a.BaseBranch,
		SkipEnv:     a.SkipEnv,
		EnvStrategy: string(a.EnvStrategy),
		EnvFiles:    a.EnvFiles,
		SkipHooks:   a.SkipHooks,
		OnCreate:    a.OnCreate,
		SkipClean:   a.SkipClean,
		OnClean:     a.OnClean,
	}
}

// configToTemplate renders an empty env strategy as a commented (skipped)
// section so the file stays valid.
func configToTemplate(c domain.ProjectConfig) projectTemplateData {
	return projectTemplateData{
		BasePath:    c.Worktrees.BasePath,
		BaseBranch:  c.Worktrees.BaseBranch,
		SkipEnv:     c.Env.Strategy == "",
		EnvStrategy: string(c.Env.Strategy),
		EnvFiles:    c.Env.Files,
		SkipHooks:   false,
		OnCreate:    c.Hooks.OnCreate,
		SkipClean:   false,
		OnClean:     c.Hooks.OnClean,
		Remembered:  c.Wizard.Remembered,
	}
}

type WriteRunParams struct {
	StateDir string
	Config   domain.RunConfig
	Force    bool // overwrite run.toml if it already exists
}

func WriteRun(ctx context.Context, params WriteRunParams) error {
	path := filepath.Join(params.StateDir, domain.RunFileName)
	_, err := fs.UpdateFile(ctx, fs.UpdateFileParams{
		Path: path,
		Mutate: func(current []byte) ([]byte, error) {
			if current != nil && !params.Force {
				return nil, ErrRunFileExists
			}
			if err := refuseNewer(refuseNewerParams{Path: path, Current: current, Supported: domainstatefile.RunConfigSchema}); err != nil {
				return nil, err
			}
			var buf bytes.Buffer
			buf.WriteString("#:schema ./schemas/run.schema.json\n\n")
			if err := toml.NewEncoder(&buf).Encode(runFileOf(params.Config)); err != nil {
				return nil, fmt.Errorf("encode run config: %w", err)
			}
			return buf.Bytes(), nil
		},
	})
	if err != nil {
		return err
	}
	return writeSchema(ctx, writeSchemaParams{Dir: params.StateDir, Schema: schemas.Run})
}

// runFile is run.toml as it is written. It exists for the settings whose unset
// value is zero: the TOML encoder does not honour `omitempty` on a scalar int,
// so a plain encode wrote `port_offset_block = 0` into every generated file — a
// value the loader ignores and the file's own schema rejects, since it requires
// a minimum of 1. A pointer is empty in the way the encoder understands.
type runFile struct {
	Schema           *int               `toml:"schema,omitempty"`
	PortOffsetBlock  *int               `toml:"port_offset_block,omitempty"`
	PortProbeTimeout *int               `toml:"port_probe_timeout,omitempty"`
	Addressing       domain.Addressing  `toml:"addressing,omitempty"`
	Concurrency      domain.Concurrency `toml:"concurrency,omitempty"`
	Isolation        domain.Isolation   `toml:"isolation,omitempty"`

	Jobs      []domain.JobConfig     `toml:"job"`
	Profiles  []domain.ProfileConfig `toml:"profile,omitempty"`
	EnvPorts  []domain.EnvPortLink   `toml:"env_port,omitempty"`
	EnvValues []domain.EnvValueLink  `toml:"env,omitempty"`
}

func runFileOf(cfg domain.RunConfig) runFile {
	file := runFile{
		Schema:           nonZero(statefile.Written(domainstatefile.RunConfigSchema)),
		PortOffsetBlock:  nonZero(cfg.PortOffsetBlock),
		PortProbeTimeout: nonZero(cfg.PortProbeTimeout),
		Addressing:       cfg.Addressing,
		Concurrency:      cfg.Concurrency,
		Isolation:        cfg.Isolation,
		Jobs:             cfg.Jobs,
		Profiles:         cfg.Profiles,
		EnvPorts:         cfg.EnvPorts,
		EnvValues:        cfg.EnvValues,
	}
	return file
}

func nonZero(value int) *int {
	if value == 0 {
		return nil
	}
	return &value
}

func WriteGlobal(ctx context.Context, answers domain.InitGlobalAnswers) error {
	dir, err := fs.GlobalDir()
	if err != nil {
		return err
	}
	return writeGlobalAt(ctx, writeGlobalAtParams{Path: filepath.Join(dir, domain.GlobalConfigFile), Answers: answers})
}

type writeGlobalAtParams struct {
	Path    string
	Answers domain.InitGlobalAnswers
}

func writeGlobalAt(ctx context.Context, params writeGlobalAtParams) error {
	_, err := fs.UpdateFile(ctx, fs.UpdateFileParams{
		Path: params.Path,
		Mutate: func(current []byte) ([]byte, error) {
			if err := refuseNewer(refuseNewerParams{Path: params.Path, Current: current, Supported: domainstatefile.GlobalConfigSchema}); err != nil {
				return nil, err
			}
			return []byte(globalContent(params.Answers)), nil
		},
	})
	if err != nil {
		return err
	}
	return writeSchema(ctx, writeSchemaParams{Dir: filepath.Dir(params.Path), Schema: schemas.Global})
}

func globalContent(answers domain.InitGlobalAnswers) string {
	header := "#:schema ./schemas/global.schema.json\n\n"
	if schema := statefile.Written(domainstatefile.GlobalConfigSchema); schema != 0 {
		header += fmt.Sprintf("schema = %d\n\n", schema)
	}
	return header + fmt.Sprintf("shell = %q\n", answers.Shell)
}
