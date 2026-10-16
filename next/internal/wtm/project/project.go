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
