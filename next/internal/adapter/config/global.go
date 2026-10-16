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

func loadGlobal(path string) (project.Global, error) {
	if path == "" {
		return project.Global{}, nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return project.Global{}, nil
	}
	if err != nil {
		return project.Global{}, err
	}
	var file globalFile
	if err := decodeStrict(strictParams{Source: path, Data: data, Out: &file}); err != nil {
		return project.Global{}, err
	}
	if file.Shell != "" && file.Shell != "zsh" && file.Shell != "bash" && file.Shell != "fish" {
		return project.Global{}, errInvalidShell
	}
	return project.Global{Proxy: project.Proxy{Port: file.Proxy.Port, Enabled: file.Proxy.Enabled}}, nil
}
