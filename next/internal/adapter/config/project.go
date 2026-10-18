// Package config reads config.toml, the global config and run.toml, strictly, and writes the answers config.toml remembers.
package config

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

const (
	configFileName = "config.toml"
	projectSchema  = 1
)

var templateSuffixes = [...]string{".example", ".dist", ".sample", ".template", ".tmpl"}

// projectFile is the shape of a project's config.toml, every key it may hold.
type projectFile struct {
	Schema    int `toml:"schema"`
	Worktrees struct {
		BasePath   string `toml:"base_path"`
		BaseBranch string `toml:"base_branch"`
	} `toml:"worktrees"`
	Env struct {
		Strategy string `toml:"strategy"`
		File     []struct {
			Target   string `toml:"target"`
			Template string `toml:"template"`
			Local    bool   `toml:"local"`
		} `toml:"file"`
	} `toml:"env"`
	Hooks  hooksFile `toml:"hooks"`
	Wizard struct {
		Remembered map[string]string `toml:"remembered"`
	} `toml:"wizard"`
}

// savedProject is config.toml as written on disk, before defaults: the config a
// use case reads and the on_clean hooks, which only a rewrite of the file needs.
type savedProject struct {
	Config  project.Config
	OnClean []project.Hook
}

// Project reads and writes the config of the project whose state lives in
// StateDir, with the global config at GlobalPath ("" for none).
type Project struct {
	StateDir   string
	GlobalPath string
}

// Load returns the project's config.toml with its defaults applied, and the
// global config. A missing config.toml is project.ErrConfigNotFound, wrapped;
// a missing global config is an empty one. An unknown key, a malformed hook or
// an invalid value in either file is an error naming it.
func (p Project) Load() (project.Config, project.Global, error) {
	path := filepath.Join(p.StateDir, configFileName)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return project.Config{}, project.Global{}, fmt.Errorf("project config: %w", project.ErrConfigNotFound)
	}
	if err != nil {
		return project.Config{}, project.Global{}, fmt.Errorf("project config: %w", err)
	}

	saved, err := parseProject(path, data)
	if err != nil {
		return project.Config{}, project.Global{}, fmt.Errorf("project config: %w", err)
	}

	global, err := loadGlobal(p.GlobalPath)
	if err != nil {
		return project.Config{}, project.Global{}, fmt.Errorf("global config: %w", err)
	}

	config := withDefaults(saved.Config)
	if err := validateProject(config); err != nil {
		return project.Config{}, project.Global{}, err
	}
	if err := global.validate(); err != nil {
		return project.Config{}, project.Global{}, err
	}

	return config, global.Global, nil
}

// parseProject decodes the config.toml read from path, strictly, without
// applying defaults nor validating values.
func parseProject(path string, data []byte) (savedProject, error) {
	var file projectFile
	if err := decodeStrict(strictParams{Source: path, Data: data, Out: &file, Ignore: hookTables}); err != nil {
		return savedProject{}, err
	}

	onCreate, onClean, err := file.Hooks.decode()
	if err != nil {
		return savedProject{}, fmt.Errorf("parse hooks: %w", err)
	}

	files := make([]project.EnvFile, len(file.Env.File))
	for i, f := range file.Env.File {
		files[i] = project.EnvFile{Target: f.Target, Template: f.Template, Local: f.Local}
	}

	return savedProject{
		Config: project.Config{
			Worktrees:  project.Worktrees{BasePath: file.Worktrees.BasePath, BaseBranch: file.Worktrees.BaseBranch},
			Env:        project.Env{Strategy: project.EnvStrategy(file.Env.Strategy), Files: files},
			OnCreate:   onCreate,
			Remembered: file.Wizard.Remembered,
		},
		OnClean: onClean,
	}, nil
}

func withDefaults(config project.Config) project.Config {
	config.Worktrees.BasePath = cmp.Or(config.Worktrees.BasePath, project.DefaultBasePath)
	config.Worktrees.BaseBranch = cmp.Or(config.Worktrees.BaseBranch, project.DefaultBaseBranch)
	config.Env.Strategy = cmp.Or(config.Env.Strategy, project.DefaultEnvStrategy)
	return config
}

var (
	errInvalidEnvStrategy = errors.New("invalid env strategy: must be example, main, or parent")
	errAbsoluteBasePath   = errors.New("base_path must be relative to the repository")
	errEnvFileNoTarget    = errors.New("env file entry must have a target")
	errEnvFileDuplicate   = errors.New("duplicate env file target")
	errEnvFileBadTemplate = errors.New("env file template must be a known template of its target")
)

func validateProject(config project.Config) error {
	if !config.Env.Strategy.Valid() {
		return errInvalidEnvStrategy
	}

	// An absolute path would be joined under the main checkout all the same (worktree.Repository.PathOf).
	if filepath.IsAbs(config.Worktrees.BasePath) {
		return fmt.Errorf("%w: %s", errAbsoluteBasePath, config.Worktrees.BasePath)
	}

	if err := validateEnvFiles(config.Env.Files); err != nil {
		return err
	}

	return validateRemembered(config.Remembered)
}

// validateEnvFiles refuses an env file without a target, a target declared
// twice, and a template that is not its target with a known template suffix.
func validateEnvFiles(files []project.EnvFile) error {
	seen := make(map[string]bool, len(files))
	for _, file := range files {
		switch {
		case file.Target == "":
			return errEnvFileNoTarget
		case seen[file.Target]:
			return fmt.Errorf("%w: %s", errEnvFileDuplicate, file.Target)
		case file.Template != "" && !isTemplateOf(file):
			return fmt.Errorf("%w: %s is not a template of %s", errEnvFileBadTemplate, file.Template, file.Target)
		}
		seen[file.Target] = true
	}
	return nil
}

// isTemplateOf reports whether file's template is its target followed by one
// of the known template suffixes.
func isTemplateOf(file project.EnvFile) bool {
	return slices.ContainsFunc(templateSuffixes[:], func(suffix string) bool { return file.Target+suffix == file.Template })
}

// validateRemembered refuses a remembered answer whose question is unknown or
// whose value is not one that question may remember, checked in sorted order
// so the first error is the same on every run.
func validateRemembered(remembered map[string]string) error {
	for _, id := range slices.Sorted(maps.Keys(remembered)) {
		allowed, known := project.RememberableValues(id)
		if !known {
			return fmt.Errorf("unknown remembered answer %q in [wizard.remembered] (expected one of %s)", id, strings.Join(project.RememberIDs(), ", "))
		}
		if !slices.Contains(allowed, remembered[id]) {
			return fmt.Errorf("invalid remembered %s %q in [wizard.remembered] (expected one of %s)", id, remembered[id], strings.Join(allowed, ", "))
		}
	}
	return nil
}
