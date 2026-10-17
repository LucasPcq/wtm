// Package project is what a use case reads of config.toml, run.toml and the global settings.
package project

import (
	"errors"
	"fmt"
	"slices"
)

var ErrConfigNotFound = errors.New("config file not found")

const (
	DefaultBasePath   = "../.trees"
	DefaultBaseBranch = "main"
)

type Config struct {
	Worktrees  Worktrees
	Env        Env
	OnCreate   []Hook
	Remembered map[string]string
}

type Worktrees struct {
	BasePath   string
	BaseBranch string
}

type Env struct {
	Strategy EnvStrategy
	Files    []EnvFile
}

type EnvFile struct {
	Target   string
	Template string
	Local    bool
}

type Hook struct {
	Cmd             string
	Cwd             string
	ContinueOnError bool
}

type Global struct {
	Proxy Proxy
}

type Proxy struct {
	Port    int
	Enabled *bool
}

const defaultProxyPort = 11080

func (g Global) ProxyPort() int {
	if g.Proxy.Enabled != nil && !*g.Proxy.Enabled {
		return 0
	}
	if g.Proxy.Port > 0 {
		return g.Proxy.Port
	}
	return defaultProxyPort
}

const (
	portMin = 1
	portMax = 65535
)

func (p Proxy) Problem() error {
	if p.Port == 0 || (p.Port >= portMin && p.Port <= portMax) {
		return nil
	}
	return fmt.Errorf("invalid [proxy] port %d in config.toml — use %d-%d, or leave it out for %d", p.Port, portMin, portMax, defaultProxyPort)
}

type EnvStrategy string

const (
	EnvExample EnvStrategy = "example"
	EnvMain    EnvStrategy = "main"
	EnvParent  EnvStrategy = "parent"
)

const DefaultEnvStrategy = EnvExample

func EnvStrategies() []EnvStrategy {
	return []EnvStrategy{EnvExample, EnvMain, EnvParent}
}

func (s EnvStrategy) Valid() bool {
	return slices.Contains(EnvStrategies(), s)
}

const (
	RememberEnvStrategy  = "env_strategy"
	RememberIsolation    = "isolation"
	RememberSourceUpdate = "source_update"

	SourceUpdateFastForward = "ff"
	SourceUpdateKeep        = "keep"
)

func RememberableValues(id string) ([]string, bool) {
	switch id {
	case RememberEnvStrategy:
		return []string{string(EnvExample), string(EnvMain), string(EnvParent)}, true
	case RememberIsolation:
		return []string{"isolated", "verbatim"}, true
	case RememberSourceUpdate:
		return []string{SourceUpdateFastForward, SourceUpdateKeep}, true
	}
	return nil, false
}

func RememberIDs() []string {
	return []string{RememberEnvStrategy, RememberIsolation, RememberSourceUpdate}
}
