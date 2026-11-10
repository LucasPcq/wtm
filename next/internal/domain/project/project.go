// Package project is what a use case reads of config.toml, run.toml and the global settings.
package project

import (
	"errors"
	"fmt"
	"slices"
)

// ErrConfigNotFound is returned when the project has no config.toml: wtm init has not run.
var ErrConfigNotFound = errors.New("config file not found")

// ErrConfigChanged refuses a write of config.toml whose file changed on disk since it was read.
var ErrConfigChanged = errors.New("config.toml changed on disk since it was read")

// ConfigSchema is the newest schema of config.toml this wtm writes; a file without one is at schema 1.
const ConfigSchema = 1

// Saved is config.toml as written on disk, before any default applies and before its values are checked, the digest of its bytes, which a write of it compares with the file as it then stands, and the schema it carries (0 when it names none).
type Saved struct {
	Config Config
	Digest string
	Schema int
}

// Newer reports whether a newer wtm wrote the file: it reads, but this wtm does not rewrite it.
func (s Saved) Newer() bool {
	return s.Schema > ConfigSchema
}

// Rewrite is config.toml written whole: its settings, and the digest of the file it replaces, as Saved read it ("" when there was none).
type Rewrite struct {
	Config   Config
	Replaces string
}

// An InvalidConfigError is a config.toml that does not parse or does not validate; Reason says why, and Line, Column and Key where the parse stopped (0 and "" when it does not say).
type InvalidConfigError struct {
	Reason error
	Line   int
	Column int
	Key    string
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

// Config is the project's config.toml: where worktrees go, how their env is made, what runs on creation and before a removal, and the wizard's remembered answers by question ID.
type Config struct {
	Worktrees  Worktrees
	Env        Env
	OnCreate   []Hook
	OnClean    []Hook
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
	Target   string `json:"target"`
	Template string `json:"template,omitempty"`
	Local    bool   `json:"local,omitempty"`
}

// Hook is one command run in a new worktree: its command line, its directory relative to the worktree, and whether a failure lets the next hooks run.
type Hook struct {
	Cmd             string `json:"cmd"`
	Cwd             string `json:"cwd,omitempty"`
	ContinueOnError bool   `json:"continue_on_error,omitempty"`
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

// EnvNone is the answer to where a new worktree's .env files come from that copies none: config.toml then names neither a strategy nor a file. It is no EnvStrategy: a worktree's record, a plan or config.toml never holds it.
const EnvNone = "none"

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
