package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

const globalConfigDir = "wtm"

var errInvalidShell = errors.New("invalid shell type: must be zsh, bash, or fish")

// globalFile is the shape of the global config.toml, every key it may hold.
type globalFile struct {
	Schema int    `toml:"schema"`
	Shell  string `toml:"shell"`
	UI     struct {
		Animations *bool `toml:"animations"`
	} `toml:"ui"`
	Proxy struct {
		Port    int   `toml:"port"`
		Enabled *bool `toml:"enabled"`
	} `toml:"proxy"`
	Update struct {
		Check *bool `toml:"check"`
	} `toml:"update"`
}

// GlobalPath returns the path of the user's global config.toml, under the
// user's config directory, or "" when that directory is unknown.
func GlobalPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}

	return filepath.Join(dir, globalConfigDir, configFileName)
}

// loadedGlobal is the global config as read: the settings a use case sees and
// the shell, kept only to be validated.
type loadedGlobal struct {
	Global project.Global
	Shell  string
}

func (g loadedGlobal) validate() error {
	switch g.Shell {
	case "", "zsh", "bash", "fish":
		return nil
	}
	return errInvalidShell
}

// LoadGlobal reads and validates the global config at path; an empty path or
// a missing file is an empty config, not an error.
func LoadGlobal(path string) (project.Global, error) {
	global, err := loadGlobal(path)
	if err != nil {
		return project.Global{}, fmt.Errorf("global config: %w", err)
	}
	if err := global.validate(); err != nil {
		return project.Global{}, err
	}
	if err := global.Global.Proxy.Problem(); err != nil {
		return project.Global{}, err
	}

	return global.Global, nil
}

// loadGlobal reads the global config at path; an empty path or a missing file
// is an empty config, not an error.
func loadGlobal(path string) (loadedGlobal, error) {
	if path == "" {
		return loadedGlobal{}, nil
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return loadedGlobal{}, nil
	}
	if err != nil {
		return loadedGlobal{}, err
	}

	var file globalFile
	if err := decodeStrict(strictParams{Source: path, Data: data, Out: &file}); err != nil {
		return loadedGlobal{}, err
	}

	return loadedGlobal{Global: project.Global{Proxy: project.Proxy{Port: file.Proxy.Port, Enabled: file.Proxy.Enabled}}, Shell: file.Shell}, nil
}
