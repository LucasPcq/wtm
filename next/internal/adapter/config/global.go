package config

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/LucasPcq/wtm/next/internal/wtm/project"
)

const globalConfigDir = "wtm"

var errInvalidShell = errors.New("invalid shell type: must be zsh, bash, or fish")

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

func GlobalPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, globalConfigDir, configFileName)
}

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
