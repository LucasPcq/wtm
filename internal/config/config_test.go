package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

const fullToml = `
[worktrees]
base_path = ".trees"
base_branch = "develop"

[env]
strategy = "main"

[[env.file]]
target = ".env"

[[env.file]]
target = "apps/api/.env"

[hooks]
on_create = [
  "pnpm install",
  { cmd = "pnpm install", cwd = "apps/api" },
]
`

const minimalToml = `
[worktrees]
`

func writeFile(t *testing.T, dir string, name string, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadFullConfig(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, domain.ConfigFileName, fullToml)

	cfg, err := Load(LoadParams{StateDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Project.Worktrees.BaseBranch != "develop" {
		t.Errorf("expected base_branch=develop, got %s", cfg.Project.Worktrees.BaseBranch)
	}
	if cfg.Project.Env.Strategy != domain.EnvStrategyMain {
		t.Errorf("expected strategy=main, got %s", cfg.Project.Env.Strategy)
	}
	if len(cfg.Project.Env.Files) != 2 || cfg.Project.Env.Files[0].Target != ".env" {
		t.Errorf("env files not loaded: %+v", cfg.Project.Env.Files)
	}
}

func TestLoadStructuredEnvFiles(t *testing.T) {
	dir := t.TempDir()
	toml := "[env]\nstrategy = \"example\"\n\n[[env.file]]\ntarget = \".env\"\ntemplate = \".env.dist\"\n\n[[env.file]]\ntarget = \".env.local\"\nlocal = true\n"
	writeFile(t, dir, domain.ConfigFileName, toml)

	cfg, err := Load(LoadParams{StateDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	env, ok := findEnvFile(cfg.Project.Env.Files, ".env")
	if !ok || env.Template != ".env.dist" {
		t.Errorf("structured template not read: %+v", cfg.Project.Env.Files)
	}
	local, ok := findEnvFile(cfg.Project.Env.Files, ".env.local")
	if !ok || !local.Local {
		t.Errorf(".env.local not flagged local: %+v", cfg.Project.Env.Files)
	}
}

func findEnvFile(files []domain.EnvFile, target string) (domain.EnvFile, bool) {
	for _, f := range files {
		if f.Target == target {
			return f, true
		}
	}
	return domain.EnvFile{}, false
}

func TestLoadMinimalConfigAppliesDefaults(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, domain.ConfigFileName, minimalToml)

	cfg, err := Load(LoadParams{StateDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Project.Worktrees.BasePath != domain.DefaultBasePath {
		t.Errorf("expected default base_path=%s, got %s", domain.DefaultBasePath, cfg.Project.Worktrees.BasePath)
	}
	if cfg.Project.Worktrees.BaseBranch != domain.DefaultBaseBranch {
		t.Errorf("expected default base_branch=%s, got %s", domain.DefaultBaseBranch, cfg.Project.Worktrees.BaseBranch)
	}
	if cfg.Project.Env.Strategy != domain.DefaultEnvStrategy {
		t.Errorf("expected default strategy=%s, got %s", domain.DefaultEnvStrategy, cfg.Project.Env.Strategy)
	}
	if cfg.Global.Shell != domain.DefaultShell {
		t.Errorf("expected default shell=%s, got %s", domain.DefaultShell, cfg.Global.Shell)
	}
}

func TestGlobalConfigDecodesUIAnimations(t *testing.T) {
	var cfg domain.GlobalConfig
	if err := decodeStrictBytes("config.toml", []byte("shell = \"zsh\"\n\n[ui]\nanimations = false\n"), &cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.UI.Animations == nil || *cfg.UI.Animations != false {
		t.Errorf("UI.Animations = %v, want a pointer to false", cfg.UI.Animations)
	}
}

func TestGlobalConfigLeavesUIAnimationsNilWhenAbsent(t *testing.T) {
	var cfg domain.GlobalConfig
	if err := decodeStrictBytes("config.toml", []byte("shell = \"zsh\"\n"), &cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.UI.Animations != nil {
		t.Errorf("UI.Animations = %v, want nil: an absent key must not read as an explicit false", cfg.UI.Animations)
	}
}

func TestGlobalConfigDecodesProxy(t *testing.T) {
	var cfg domain.GlobalConfig
	if err := decodeStrictBytes("config.toml", []byte("shell = \"zsh\"\n\n[proxy]\nport = 5000\n"), &cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Proxy.Port != 5000 {
		t.Errorf("Proxy.Port = %d, want 5000", cfg.Proxy.Port)
	}
	if cfg.Proxy.Enabled != nil {
		t.Errorf("Proxy.Enabled = %v, want nil: an absent key must not read as an explicit answer", cfg.Proxy.Enabled)
	}
}

func TestGlobalConfigDecodesProxyDisabled(t *testing.T) {
	var cfg domain.GlobalConfig
	if err := decodeStrictBytes("config.toml", []byte("[proxy]\nenabled = false\n"), &cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Proxy.Enabled == nil || *cfg.Proxy.Enabled {
		t.Errorf("Proxy.Enabled = %v, want a pointer to false", cfg.Proxy.Enabled)
	}
}

func TestLoadMissingProjectConfigReturnsError(t *testing.T) {
	dir := t.TempDir()

	_, err := Load(LoadParams{StateDir: dir})
	if !errors.Is(err, domain.ErrConfigNotFound) {
		t.Errorf("expected ErrConfigNotFound, got %v", err)
	}
}

func TestHookCommandStringParsing(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, domain.ConfigFileName, `
[hooks]
on_create = ["pnpm install"]
`)

	cfg, err := Load(LoadParams{StateDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Project.Hooks.OnCreate) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(cfg.Project.Hooks.OnCreate))
	}

	hook := cfg.Project.Hooks.OnCreate[0]
	if hook.Cmd != "pnpm install" {
		t.Errorf("expected cmd=pnpm install, got %s", hook.Cmd)
	}
	if hook.Cwd != "" {
		t.Errorf("expected empty cwd, got %s", hook.Cwd)
	}
}

func TestHookCommandObjectParsing(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, domain.ConfigFileName, `
[hooks]
on_create = [
  { cmd = "pnpm install", cwd = "apps/api" },
]
`)

	cfg, err := Load(LoadParams{StateDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Project.Hooks.OnCreate) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(cfg.Project.Hooks.OnCreate))
	}

	hook := cfg.Project.Hooks.OnCreate[0]
	if hook.Cmd != "pnpm install" {
		t.Errorf("expected cmd=pnpm install, got %s", hook.Cmd)
	}
	if hook.Cwd != "apps/api" {
		t.Errorf("expected cwd=apps/api, got %s", hook.Cwd)
	}
}

func TestHookCommandMixedParsing(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, domain.ConfigFileName, fullToml)

	cfg, err := Load(LoadParams{StateDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hooks := cfg.Project.Hooks.OnCreate
	if len(hooks) != 2 {
		t.Fatalf("expected 2 on_create hooks, got %d", len(hooks))
	}

	if hooks[0].Cmd != "pnpm install" || hooks[0].Cwd != "" {
		t.Errorf("hook[0]: expected {cmd:pnpm install, cwd:}, got {cmd:%s, cwd:%s}", hooks[0].Cmd, hooks[0].Cwd)
	}
	if hooks[1].Cmd != "pnpm install" || hooks[1].Cwd != "apps/api" {
		t.Errorf("hook[1]: expected {cmd:pnpm install, cwd:apps/api}, got {cmd:%s, cwd:%s}", hooks[1].Cmd, hooks[1].Cwd)
	}
}

func TestLoad_CorruptedTOML(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, domain.ConfigFileName, `
[worktrees
this is not valid toml !!!
`)

	_, err := Load(LoadParams{StateDir: dir})
	if err == nil {
		t.Fatal("expected error for corrupted TOML, got nil")
	}
}

func TestApplyDefaults_AllEmpty(t *testing.T) {
	cfg := domain.Config{}
	applyDefaults(&cfg)

	if cfg.Project.Worktrees.BasePath != domain.DefaultBasePath {
		t.Errorf("expected BasePath=%s, got %s", domain.DefaultBasePath, cfg.Project.Worktrees.BasePath)
	}
	if cfg.Project.Worktrees.BaseBranch != domain.DefaultBaseBranch {
		t.Errorf("expected BaseBranch=%s, got %s", domain.DefaultBaseBranch, cfg.Project.Worktrees.BaseBranch)
	}
	if cfg.Project.Env.Strategy != domain.DefaultEnvStrategy {
		t.Errorf("expected Strategy=%s, got %s", domain.DefaultEnvStrategy, cfg.Project.Env.Strategy)
	}
	if cfg.Global.Shell != domain.DefaultShell {
		t.Errorf("expected Shell=%s, got %s", domain.DefaultShell, cfg.Global.Shell)
	}
}
