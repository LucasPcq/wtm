// Package project is what a use case reads of config.toml, run.toml and the global settings.
package project

import (
	"errors"
	"fmt"
	"slices"
)

// ErrConfigNotFound is returned when the project has no config.toml: wtm init has not run.
var ErrConfigNotFound = errors.New("config file not found")

// An InvalidConfigError is a config.toml that does not parse or does not validate; Reason says why.
type InvalidConfigError struct {
	Reason error
}

// Error is the reason.
func (e *InvalidConfigError) Error() string {
	return e.Reason.Error()
}

// Unwrap returns the reason, so errors.Is and errors.As see through it.
func (e *InvalidConfigError) Unwrap() error {
	return e.Reason
}

// The defaults of config.toml's [worktrees] section.
const (
	DefaultBasePath   = "../.trees"
	DefaultBaseBranch = "main"
)

// Config is the project's config.toml: where worktrees go, how their env is made, what runs on creation, and the wizard's remembered answers by question ID.
type Config struct {
	Worktrees  Worktrees
	Env        Env
	OnCreate   []Hook
	Remembered map[string]string
}

// Worktrees is where new worktrees go, relative to the main one, and the branch they start from by default.
type Worktrees struct {
	BasePath   string
	BaseBranch string
}

// Env is how a new worktree's .env files are made: the strategy that picks their source, and the files.
type Env struct {
	Strategy EnvStrategy
	Files    []EnvFile
}

// EnvFile is one .env file a worktree gets: its path in the worktree, the template it starts from (empty to look for the target with a conventional suffix, `.example` first), and config.toml's `local` key.
type EnvFile struct {
	Target   string
	Template string
	Local    bool
}

// Hook is one command run in a new worktree: its command line, its directory relative to the worktree, and whether a failure lets the next hooks run.
type Hook struct {
	Cmd             string
	Cwd             string
	ContinueOnError bool
}

// Global is the settings shared by every project of the user.
type Global struct {
	Proxy Proxy
}

// Proxy is the local proxy's settings: its port (0 for the default) and whether it is on (nil for the default, on).
type Proxy struct {
	Port    int
	Enabled *bool
}

const defaultProxyPort = 11080

// ProxyPort returns the port the proxy listens on, 0 when it is disabled.
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

// Problem returns why the proxy's port is invalid, or nil when it is unset or a valid port.
func (p Proxy) Problem() error {
	if p.Port == 0 || (p.Port >= portMin && p.Port <= portMax) {
		return nil
	}

	return fmt.Errorf("invalid [proxy] port %d in config.toml — use %d-%d, or leave it out for %d", p.Port, portMin, portMax, defaultProxyPort)
}

// EnvStrategy is where a new worktree's .env files are copied from.
type EnvStrategy string

// The env strategies.
const (
	EnvExample EnvStrategy = "example" // each file's template
	EnvMain    EnvStrategy = "main"    // the main worktree's files
	EnvParent  EnvStrategy = "parent"  // the files of the worktree the new one starts from
)

// DefaultEnvStrategy is the strategy when neither the request nor config.toml names one.
const DefaultEnvStrategy = EnvExample

// EnvStrategies returns every env strategy, in the order a choice offers them.
func EnvStrategies() []EnvStrategy {
	return []EnvStrategy{EnvExample, EnvMain, EnvParent}
}

// Valid reports whether s is one of EnvStrategies.
func (s EnvStrategy) Valid() bool {
	return slices.Contains(EnvStrategies(), s)
}

// The IDs of the questions whose answer the wizard can remember, and the values of the source-update question.
const (
	RememberEnvStrategy  = "env_strategy"
	RememberIsolation    = "isolation"
	RememberSourceUpdate = "source_update"

	SourceUpdateFastForward = "ff"
	SourceUpdateKeep        = "keep"

	// RememberEnvConfig is the remembered env strategy that follows env.strategy, as the legacy wizard writes its "Use config default" row.
	RememberEnvConfig = "config"
)

// RememberableValues returns the values the question id may remember, and reports whether id is a question that remembers.
func RememberableValues(id string) ([]string, bool) {
	switch id {
	case RememberEnvStrategy:
		return []string{RememberEnvConfig, string(EnvExample), string(EnvMain), string(EnvParent)}, true
	case RememberIsolation:
		return []string{"isolated", "verbatim"}, true
	case RememberSourceUpdate:
		return []string{SourceUpdateFastForward, SourceUpdateKeep}, true
	}

	return nil, false
}

// RememberIDs returns the IDs of the questions whose answer the wizard can remember.
func RememberIDs() []string {
	return []string{RememberEnvStrategy, RememberIsolation, RememberSourceUpdate}
}

// SourceUpdate is the remembered answer of the source-update question for whether the source is fast-forwarded first.
func SourceUpdate(fastForward bool) string {
	if fastForward {
		return SourceUpdateFastForward
	}

	return SourceUpdateKeep
}
