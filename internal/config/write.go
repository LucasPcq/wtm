package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/schemas"
)

// ErrRunFileExists is returned by WriteRun when the target file already
// exists — callers decide whether to skip or surface the condition.
var ErrRunFileExists = errors.New("run file already exists")

// projectTemplateData is the unified view rendered by the config template.
// Both the init wizard answers and a full ProjectConfig (for re-init) convert
// to it, so the template emits every section's actual values.
type projectTemplateData struct {
	BasePath    string
	BaseBranch  string
	SkipEnv     bool
	EnvStrategy string
	EnvFiles    []domain.EnvFile
	SkipHooks   bool
	OnCreate    []domain.HookCommand
	SkipClean   bool
	OnClean     []domain.HookCommand
}

// WriteProjectParams holds the inputs for writing a project config file.
type WriteProjectParams struct {
	StateDir string
	Answers  domain.InitProjectAnswers
}

// WriteProject renders the project config from init wizard answers and writes it
// to <state-dir>/config.toml.
func WriteProject(params WriteProjectParams) error {
	return renderProjectConfig(params.StateDir, answersToTemplate(params.Answers))
}

// WriteProjectConfigParams holds the inputs for rewriting config.toml from a
// full ProjectConfig (targeted re-init).
type WriteProjectConfigParams struct {
	StateDir string
	Config   domain.ProjectConfig
}

// WriteProjectConfig re-renders config.toml from a full ProjectConfig, preserving
// every section's current values (only manual comments are regenerated).
func WriteProjectConfig(params WriteProjectConfigParams) error {
	return renderProjectConfig(params.StateDir, configToTemplate(params.Config))
}

// renderProjectConfig renders the template data and writes config.toml.
func renderProjectConfig(stateDir string, data projectTemplateData) error {
	var buf bytes.Buffer
	if err := parsedTemplate.Execute(&buf, data); err != nil {
		return fmt.Errorf("render template: %w", err)
	}

	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", stateDir, err)
	}

	path := filepath.Join(stateDir, domain.ConfigFileName)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	return writeSchema(stateDir, schemas.Project)
}

// writeSchema puts this binary's schema beside the file that points at it, on
// every write rather than once at init: an upgrade otherwise leaves the editor
// validating against the version the repository was first set up with. Loading
// writes nothing — `wtm schema dump` refreshes them without touching a config.
func writeSchema(dir string, schema schemas.Schema) error {
	schemaDir := filepath.Join(dir, domain.SchemasDirName)
	if err := os.MkdirAll(schemaDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", schemaDir, err)
	}
	path := filepath.Join(schemaDir, schema.Filename())
	if err := os.WriteFile(path, schema.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// answersToTemplate converts init wizard answers to template data.
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

// configToTemplate converts a full ProjectConfig to template data for re-init.
// An empty env strategy is rendered as a commented (skipped) section so the file
// stays valid.
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
	}
}

// WriteRunParams holds the inputs for writing a run config file.
type WriteRunParams struct {
	StateDir string
	Config   domain.RunConfig
	Force    bool // overwrite run.toml if it already exists
}

// WriteRun encodes cfg as TOML and writes it to <state-dir>/run.toml.
// Returns ErrRunFileExists if the file already exists and Force is false.
func WriteRun(params WriteRunParams) error {
	path := filepath.Join(params.StateDir, domain.RunFileName)

	if _, err := os.Stat(path); err == nil && !params.Force {
		return ErrRunFileExists
	}

	if err := os.MkdirAll(params.StateDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", params.StateDir, err)
	}

	var buf bytes.Buffer
	buf.WriteString("#:schema ./schemas/run.schema.json\n\n")
	if err := toml.NewEncoder(&buf).Encode(runFileOf(params.Config)); err != nil {
		return fmt.Errorf("encode run config: %w", err)
	}

	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	return writeSchema(params.StateDir, schemas.Run)
}

// runFile is run.toml as it is written. It exists for the two settings whose
// unset value is zero: the TOML encoder does not honour `omitempty` on a scalar
// int, so a plain encode wrote `port_offset_block = 0` into every generated
// file — a value the loader ignores and the file's own schema rejects, since it
// requires a minimum of 1. A pointer is empty in the way the encoder
// understands.
type runFile struct {
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
		Addressing:  cfg.Addressing,
		Concurrency: cfg.Concurrency,
		Isolation:   cfg.Isolation,
		Jobs:        cfg.Jobs,
		Profiles:    cfg.Profiles,
		EnvPorts:    cfg.EnvPorts,
		EnvValues:   cfg.EnvValues,
	}
	if cfg.PortOffsetBlock != 0 {
		file.PortOffsetBlock = &cfg.PortOffsetBlock
	}
	if cfg.PortProbeTimeout != 0 {
		file.PortProbeTimeout = &cfg.PortProbeTimeout
	}
	return file
}

// WriteGlobal creates the global config directory and writes config.toml.
func WriteGlobal(answers domain.InitGlobalAnswers) error {
	dir, err := infra.GlobalDir()
	if err != nil {
		return err
	}
	return writeGlobalAt(filepath.Join(dir, domain.GlobalConfigFile), answers)
}

func writeGlobalAt(path string, answers domain.InitGlobalAnswers) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	content := fmt.Sprintf("#:schema ./schemas/global.schema.json\n\nshell = %q\n", answers.Shell)

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	return writeSchema(dir, schemas.Global)
}
