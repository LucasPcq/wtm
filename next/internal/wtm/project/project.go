// Package project is what a slice reads of a repository's config.toml and of the global settings.
package project

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
