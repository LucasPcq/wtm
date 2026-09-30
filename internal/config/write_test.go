package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/LucasPcq/wtm/internal/domain"
)

func TestWriteProjectRendersValidTOML(t *testing.T) {
	dir := t.TempDir()

	answers := domain.InitProjectAnswers{
		BasePath:   ".trees",
		BaseBranch: "main",
		EnvFiles: []domain.EnvFile{
			{Target: ".env", Template: ".env.example"},
			{Target: "apps/api/.env", Template: "apps/api/.env.example"},
		},
		EnvStrategy: domain.EnvStrategyExample,
		OnCreate:    []domain.HookCommand{{Cmd: "pnpm install"}},
	}

	err := WriteProject(WriteProjectParams{
		StateDir: dir,
		Answers:  answers,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(dir, domain.ConfigFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	content := string(data)

	// Verify it contains expected values
	if !strings.Contains(content, `base_path = ".trees"`) {
		t.Error("missing base_path")
	}
	if !strings.Contains(content, `strategy = "example"`) {
		t.Error("missing strategy")
	}
	if !strings.Contains(content, `"pnpm install"`) {
		t.Error("missing install command in hooks")
	}
	if !strings.Contains(content, "[[env.file]]") || !strings.Contains(content, `target = ".env"`) {
		t.Errorf("missing structured env file block:\n%s", content)
	}
	if !strings.Contains(content, `template = ".env.example"`) {
		t.Errorf("missing pinned template:\n%s", content)
	}

	// Verify it parses as valid TOML
	var raw map[string]interface{}
	if _, err := toml.Decode(content, &raw); err != nil {
		t.Fatalf("generated file is not valid TOML: %v", err)
	}
}

func TestWriteProjectEmptyEnvAndHooks(t *testing.T) {
	dir := t.TempDir()

	answers := domain.InitProjectAnswers{
		BasePath:    ".trees",
		BaseBranch:  "main",
		EnvStrategy: domain.EnvStrategyExample,
	}

	err := WriteProject(WriteProjectParams{
		StateDir: dir,
		Answers:  answers,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, domain.ConfigFileName))
	content := string(data)

	// Should still be valid TOML
	var raw map[string]interface{}
	if _, err := toml.Decode(content, &raw); err != nil {
		t.Fatalf("generated file is not valid TOML: %v", err)
	}
}

func TestWriteProjectSkipEnvCommentsSection(t *testing.T) {
	dir := t.TempDir()

	answers := domain.InitProjectAnswers{
		BasePath:   ".trees",
		BaseBranch: "main",
		SkipEnv:    true,
	}

	if err := WriteProject(WriteProjectParams{StateDir: dir, Answers: answers}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, domain.ConfigFileName))
	content := string(data)

	if strings.Contains(content, "\nstrategy = ") {
		t.Error("expected env strategy to be commented out when skipped")
	}
	if !strings.Contains(content, domain.SkipMarkerComment) {
		t.Error("expected skip marker comment in env section")
	}

	var raw map[string]interface{}
	if _, err := toml.Decode(content, &raw); err != nil {
		t.Fatalf("generated file is not valid TOML: %v", err)
	}
}

func TestWriteProjectSkipHooksCommentsSection(t *testing.T) {
	dir := t.TempDir()

	answers := domain.InitProjectAnswers{
		BasePath:    ".trees",
		BaseBranch:  "main",
		EnvStrategy: domain.EnvStrategyExample,
		SkipHooks:   true,
	}

	if err := WriteProject(WriteProjectParams{StateDir: dir, Answers: answers}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, domain.ConfigFileName))
	content := string(data)

	if !strings.Contains(content, "on_create = []") {
		t.Error("expected empty on_create when hooks skipped")
	}

	cfg, err := loadProjectConfig(filepath.Join(dir, domain.ConfigFileName))
	if err != nil {
		t.Fatalf("loadProjectConfig: %v", err)
	}
	if len(cfg.Hooks.OnCreate) != 0 {
		t.Errorf("expected no hooks, got %v", cfg.Hooks.OnCreate)
	}
}

func TestWriteProjectRoundTripTableFormHook(t *testing.T) {
	dir := t.TempDir()

	answers := domain.InitProjectAnswers{
		BasePath:    ".trees",
		BaseBranch:  "main",
		EnvStrategy: domain.EnvStrategyExample,
		OnCreate: []domain.HookCommand{
			{Cmd: "pnpm install"},
			{Cmd: "pnpm db:migrate", Cwd: "apps/api", ContinueOnError: true},
		},
	}

	if err := WriteProject(WriteProjectParams{StateDir: dir, Answers: answers}); err != nil {
		t.Fatalf("WriteProject: %v", err)
	}

	cfg, err := loadProjectConfig(filepath.Join(dir, domain.ConfigFileName))
	if err != nil {
		t.Fatalf("loadProjectConfig: %v", err)
	}

	if len(cfg.Hooks.OnCreate) != 2 {
		t.Fatalf("expected 2 hooks, got %d: %+v", len(cfg.Hooks.OnCreate), cfg.Hooks.OnCreate)
	}
	if cfg.Hooks.OnCreate[0].Cmd != "pnpm install" || cfg.Hooks.OnCreate[0].Cwd != "" {
		t.Errorf("bare hook not preserved: %+v", cfg.Hooks.OnCreate[0])
	}
	table := cfg.Hooks.OnCreate[1]
	if table.Cmd != "pnpm db:migrate" || table.Cwd != "apps/api" || !table.ContinueOnError {
		t.Errorf("table-form hook fields not preserved: %+v", table)
	}
}

func TestWriteProjectSkipCleanCommentsSection(t *testing.T) {
	dir := t.TempDir()

	answers := domain.InitProjectAnswers{
		BasePath:    ".trees",
		BaseBranch:  "main",
		EnvStrategy: domain.EnvStrategyExample,
		SkipClean:   true,
	}

	if err := WriteProject(WriteProjectParams{StateDir: dir, Answers: answers}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, domain.ConfigFileName))
	content := string(data)

	if !strings.Contains(content, "on_clean = []") {
		t.Error("expected empty on_clean when clean hooks skipped")
	}

	cfg, err := loadProjectConfig(filepath.Join(dir, domain.ConfigFileName))
	if err != nil {
		t.Fatalf("loadProjectConfig: %v", err)
	}
	if len(cfg.Hooks.OnClean) != 0 {
		t.Errorf("expected no clean hooks, got %v", cfg.Hooks.OnClean)
	}
}

func TestWriteProjectRoundTripOnClean(t *testing.T) {
	dir := t.TempDir()

	answers := domain.InitProjectAnswers{
		BasePath:    ".trees",
		BaseBranch:  "main",
		EnvStrategy: domain.EnvStrategyExample,
		OnClean: []domain.HookCommand{
			{Cmd: "docker compose down"},
			{Cmd: "./scripts/teardown.sh", Cwd: "infra", ContinueOnError: true},
		},
	}

	if err := WriteProject(WriteProjectParams{StateDir: dir, Answers: answers}); err != nil {
		t.Fatalf("WriteProject: %v", err)
	}

	cfg, err := loadProjectConfig(filepath.Join(dir, domain.ConfigFileName))
	if err != nil {
		t.Fatalf("loadProjectConfig: %v", err)
	}

	if len(cfg.Hooks.OnClean) != 2 {
		t.Fatalf("expected 2 clean hooks, got %d: %+v", len(cfg.Hooks.OnClean), cfg.Hooks.OnClean)
	}
	if cfg.Hooks.OnClean[0].Cmd != "docker compose down" || cfg.Hooks.OnClean[0].Cwd != "" {
		t.Errorf("bare clean hook not preserved: %+v", cfg.Hooks.OnClean[0])
	}
	table := cfg.Hooks.OnClean[1]
	if table.Cmd != "./scripts/teardown.sh" || table.Cwd != "infra" || !table.ContinueOnError {
		t.Errorf("table-form clean hook fields not preserved: %+v", table)
	}
}

func TestWriteProjectConfigPreservesAllSections(t *testing.T) {
	dir := t.TempDir()

	cfg := domain.ProjectConfig{
		Worktrees: domain.WorktreesConfig{BasePath: "../.trees", BaseBranch: "develop"},
		Env:       domain.EnvConfig{Strategy: domain.EnvStrategyParent, Files: []domain.EnvFile{{Target: ".env"}}},
		Hooks:     domain.HooksConfig{OnCreate: []domain.HookCommand{{Cmd: "pnpm install"}}},
	}

	if err := WriteProjectConfig(WriteProjectConfigParams{StateDir: dir, Config: cfg}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := loadProjectConfig(filepath.Join(dir, domain.ConfigFileName))
	if err != nil {
		t.Fatalf("loadProjectConfig: %v", err)
	}

	if got.Env.Strategy != domain.EnvStrategyParent {
		t.Errorf("env.strategy not preserved: %q", got.Env.Strategy)
	}
	if len(got.Hooks.OnCreate) != 1 || got.Hooks.OnCreate[0].Cmd != "pnpm install" {
		t.Errorf("hooks not preserved: %+v", got.Hooks.OnCreate)
	}
}

func TestWriteProjectConfigEmptyEnvStaysCommented(t *testing.T) {
	dir := t.TempDir()

	cfg := domain.ProjectConfig{
		Worktrees: domain.WorktreesConfig{BasePath: "../.trees", BaseBranch: "main"},
		// Env.Strategy empty → section must render commented to stay valid.
	}

	if err := WriteProjectConfig(WriteProjectConfigParams{StateDir: dir, Config: cfg}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, domain.ConfigFileName))
	if strings.Contains(string(data), "\nstrategy = ") {
		t.Error("empty env strategy should be commented out, not written as empty")
	}

	var raw map[string]interface{}
	if _, err := toml.Decode(string(data), &raw); err != nil {
		t.Fatalf("generated file is not valid TOML: %v", err)
	}
}

func TestWriteRunCreatesFile(t *testing.T) {
	dir := t.TempDir()
	cfg := domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "dev", Kind: domain.JobKindService, Cmd: "pnpm dev"},
		},
	}

	if err := WriteRun(WriteRunParams{StateDir: dir, Config: cfg}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, domain.RunFileName))
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if !strings.Contains(string(data), `name = "dev"`) {
		t.Error("expected job name in output")
	}
}

func TestWriteRunRefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	cfg := domain.RunConfig{
		Jobs: []domain.JobConfig{{Name: "a", Kind: domain.JobKindService, Cmd: "echo a"}},
	}

	if err := WriteRun(WriteRunParams{StateDir: dir, Config: cfg}); err != nil {
		t.Fatalf("first write failed: %v", err)
	}
	if err := WriteRun(WriteRunParams{StateDir: dir, Config: cfg}); !errors.Is(err, ErrRunFileExists) {
		t.Errorf("expected ErrRunFileExists, got %v", err)
	}
}

func TestWriteRunForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	orig := domain.RunConfig{
		Jobs: []domain.JobConfig{{Name: "old", Kind: domain.JobKindTask, Cmd: "echo old"}},
	}
	if err := WriteRun(WriteRunParams{StateDir: dir, Config: orig}); err != nil {
		t.Fatalf("first write failed: %v", err)
	}

	updated := domain.RunConfig{
		Jobs: []domain.JobConfig{{Name: "new", Kind: domain.JobKindTask, Cmd: "echo new"}},
	}
	if err := WriteRun(WriteRunParams{StateDir: dir, Config: updated, Force: true}); err != nil {
		t.Fatalf("force write failed: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, domain.RunFileName))
	if !strings.Contains(string(data), `name = "new"`) {
		t.Error("expected updated job name after force overwrite")
	}
	if strings.Contains(string(data), `name = "old"`) {
		t.Error("expected old job to be gone after force overwrite")
	}
}

func TestWriteGlobalAt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wtm", "config.toml")

	err := writeGlobalAt(path, domain.InitGlobalAnswers{
		Shell: domain.ShellZsh,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, `shell = "zsh"`) {
		t.Error("missing shell")
	}

	// Verify it parses as valid TOML
	var cfg domain.GlobalConfig
	if _, err := toml.Decode(content, &cfg); err != nil {
		t.Fatalf("generated file is not valid TOML: %v", err)
	}
	if cfg.Shell != domain.ShellZsh {
		t.Errorf("expected shell=zsh, got %s", cfg.Shell)
	}
}

func TestWriteProjectRoundTrip(t *testing.T) {
	dir := t.TempDir()

	answers := domain.InitProjectAnswers{
		BasePath:   ".trees",
		BaseBranch: "develop",
		EnvFiles: []domain.EnvFile{
			{Target: ".env"},
			{Target: "apps/web/.env"},
		},
		EnvStrategy: domain.EnvStrategyParent,
		OnCreate:    []domain.HookCommand{{Cmd: "yarn install"}},
	}

	err := WriteProject(WriteProjectParams{
		StateDir: dir,
		Answers:  answers,
	})
	if err != nil {
		t.Fatalf("WriteProject: %v", err)
	}

	// Load it back using the existing config loader
	cfg, err := loadProjectConfig(filepath.Join(dir, domain.ConfigFileName))
	if err != nil {
		t.Fatalf("loadProjectConfig: %v", err)
	}

	if cfg.Worktrees.BasePath != ".trees" {
		t.Errorf("round-trip base_path: got %s", cfg.Worktrees.BasePath)
	}
	if cfg.Worktrees.BaseBranch != "develop" {
		t.Errorf("round-trip base_branch: got %s", cfg.Worktrees.BaseBranch)
	}
	if cfg.Env.Strategy != domain.EnvStrategyParent {
		t.Errorf("round-trip strategy: got %s", cfg.Env.Strategy)
	}
	if len(cfg.Env.Files) != 2 {
		t.Errorf("round-trip env files: got %d", len(cfg.Env.Files))
	}
	if len(cfg.Hooks.OnCreate) != 1 || cfg.Hooks.OnCreate[0].Cmd != "yarn install" {
		t.Errorf("round-trip on_create: got %v", cfg.Hooks.OnCreate)
	}
}

// The TOML encoder does not honour `omitempty` on a scalar int, so a plain
// encode wrote `port_offset_block = 0` into every generated file — a value the
// loader ignores and the file's own schema rejects.
func TestWriteRunOmitsTheSettingsThatAreUnset(t *testing.T) {
	dir := t.TempDir()
	if err := WriteRun(WriteRunParams{
		StateDir: dir,
		Force:    true,
		Config: domain.RunConfig{
			Jobs: []domain.JobConfig{{Name: "web", Kind: domain.JobKindService, Cmd: "true"}},
		},
	}); err != nil {
		t.Fatalf("write: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(dir, domain.RunFileName))
	if err != nil {
		t.Fatal(err)
	}
	for _, unwanted := range []string{"port_offset_block", "port_probe_timeout", "addressing", "concurrency"} {
		if strings.Contains(string(body), unwanted) {
			t.Errorf("file carries an unset %s:\n%s", unwanted, body)
		}
	}
}

// A value the project did set has to survive the write and come back.
func TestWriteRunKeepsTheSettingsThatAreSet(t *testing.T) {
	dir := t.TempDir()
	if err := WriteRun(WriteRunParams{
		StateDir: dir,
		Force:    true,
		Config: domain.RunConfig{
			PortOffsetBlock:  25,
			PortProbeTimeout: 42,
			Addressing:       domain.AddressingPorts,
			Concurrency:      domain.ConcurrencyExclusive,
			Jobs:             []domain.JobConfig{{Name: "web", Kind: domain.JobKindService, Cmd: "true", Ports: map[string]int{"PORT": 3000}}},
			EnvPorts:         []domain.EnvPortLink{{File: ".env", Key: "WEB_PORT", Job: "web", Port: "PORT"}},
		},
	}); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := LoadRun(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.PortOffsetBlock != 25 || got.PortProbeTimeout != 42 {
		t.Errorf("scalars lost: %+v", got)
	}
	if got.Addressing != domain.AddressingPorts || got.Concurrency != domain.ConcurrencyExclusive {
		t.Errorf("settings lost: %+v", got)
	}
	if len(got.EnvPorts) != 1 || len(got.Jobs) != 1 {
		t.Errorf("tables lost: %+v", got)
	}
}
