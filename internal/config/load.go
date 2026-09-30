// Package config loads and validates wtm configuration from project and user-level TOML files.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/rules"
)

// LoadParams holds the inputs for loading configuration.
type LoadParams struct {
	StateDir string
}

// Load reads project and global config files, merges them, applies defaults,
// and validates the result. Returns ErrConfigNotFound if config.toml is absent.
func Load(params LoadParams) (domain.Config, error) {
	projectPath := filepath.Join(params.StateDir, domain.ConfigFileName)

	project, err := loadProjectConfig(projectPath)
	if err != nil {
		return domain.Config{}, fmt.Errorf("project config: %w", err)
	}

	global, err := loadGlobalConfig()
	if err != nil {
		return domain.Config{}, fmt.Errorf("global config: %w", err)
	}

	cfg := merge(project, global)
	applyDefaults(&cfg)

	if err := rules.Validate(cfg); err != nil {
		return domain.Config{}, err
	}

	return cfg, nil
}

// LoadProjectRaw reads the project config.toml without merging the global config
// or applying defaults — the on-disk values exactly. Used by targeted re-init to
// rewrite the file while preserving every untouched section's values.
// Returns ErrConfigNotFound if the file does not exist.
func LoadProjectRaw(stateDir string) (domain.ProjectConfig, error) {
	return loadProjectConfig(filepath.Join(stateDir, domain.ConfigFileName))
}

// loadProjectConfig reads and parses the project config.toml file.
// Returns ErrConfigNotFound if the file does not exist.
func loadProjectConfig(path string) (domain.ProjectConfig, error) {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return domain.ProjectConfig{}, domain.ErrConfigNotFound
	}

	var raw rawProjectConfig
	// hooks.on_create / hooks.on_clean entries are decoded as []interface{}
	// (string|table union) so the inner table keys (cmd, cwd, continue_on_error)
	// appear as "undecoded" to the strict checker even though the custom
	// unmarshaler reads them — skip those sub-trees.
	if err := decodeStrict(path, &raw, "hooks.on_create", "hooks.on_clean"); err != nil {
		return domain.ProjectConfig{}, err
	}

	hooks, err := decodeHooksConfig(raw.Hooks)
	if err != nil {
		return domain.ProjectConfig{}, fmt.Errorf("parse hooks: %w", err)
	}

	cfg := domain.ProjectConfig{
		Worktrees: domain.WorktreesConfig{
			BasePath:   raw.Worktrees.BasePath,
			BaseBranch: raw.Worktrees.BaseBranch,
		},
		Env:   buildEnvConfig(raw.Env),
		Hooks: hooks,
	}

	return cfg, nil
}

// buildEnvConfig resolves the env config from the raw decode.
func buildEnvConfig(raw rawEnvConfig) domain.EnvConfig {
	files := make([]domain.EnvFile, len(raw.File))
	for i, f := range raw.File {
		files[i] = domain.EnvFile{Target: f.Target, Template: f.Template, Local: f.Local}
	}
	return domain.EnvConfig{
		Strategy: domain.EnvStrategy(raw.Strategy),
		Files:    files,
	}
}

// rawProjectConfig is the intermediate TOML-decoded structure.
// Hooks are decoded as raw interfaces to support the string | {cmd,cwd} union.
type rawProjectConfig struct {
	Worktrees struct {
		BasePath   string `toml:"base_path"`
		BaseBranch string `toml:"base_branch"`
	} `toml:"worktrees"`
	Env   rawEnvConfig   `toml:"env"`
	Hooks rawHooksConfig `toml:"hooks"`
}

// rawEnvConfig decodes the [env] table.
type rawEnvConfig struct {
	Strategy string       `toml:"strategy"`
	File     []rawEnvFile `toml:"file"`
}

type rawEnvFile struct {
	Target   string `toml:"target"`
	Template string `toml:"template"`
	Local    bool   `toml:"local"`
}

// GlobalPath is where the machine-level config lives, empty when the OS gives
// no config directory at all.
func GlobalPath() string {
	dir, err := infra.GlobalDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, domain.GlobalConfigFile)
}

// LoadGlobal reads the global config on its own, for the settings that belong to
// the machine rather than to a repository — a command about them must answer
// outside any wtm project.
func LoadGlobal() (domain.GlobalConfig, error) {
	return loadGlobalConfig()
}

// loadGlobalConfig returns a zero-value GlobalConfig when the file does not
// exist: the global config is optional.
func loadGlobalConfig() (domain.GlobalConfig, error) {
	path := GlobalPath()
	if path == "" {
		return domain.GlobalConfig{}, nil
	}

	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return domain.GlobalConfig{}, nil
	}

	var cfg domain.GlobalConfig
	if err := decodeStrict(path, &cfg); err != nil {
		return domain.GlobalConfig{}, err
	}

	return cfg, nil
}

// merge combines project and global configs into the unified Config.
func merge(project domain.ProjectConfig, global domain.GlobalConfig) domain.Config {
	return domain.Config{
		Project: project,
		Global:  global,
	}
}

// applyDefaults fills zero-value fields with sensible defaults.
func applyDefaults(cfg *domain.Config) {
	if cfg.Project.Worktrees.BasePath == "" {
		cfg.Project.Worktrees.BasePath = domain.DefaultBasePath
	}
	if cfg.Project.Worktrees.BaseBranch == "" {
		cfg.Project.Worktrees.BaseBranch = domain.DefaultBaseBranch
	}
	if cfg.Project.Env.Strategy == "" {
		cfg.Project.Env.Strategy = domain.DefaultEnvStrategy
	}
	if cfg.Global.Shell == "" {
		cfg.Global.Shell = domain.DefaultShell
	}
}
