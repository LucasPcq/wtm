package rules_test

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func TestBuildGlobalAnswers_Defaults(t *testing.T) {
	got, err := rules.BuildGlobalAnswers(rules.InitGlobalFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Shell != domain.DefaultShell {
		t.Errorf("Shell = %q, want default %q", got.Shell, domain.DefaultShell)
	}
}

func TestBuildGlobalAnswers_Flags(t *testing.T) {
	got, err := rules.BuildGlobalAnswers(rules.InitGlobalFlags{Shell: "fish"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Shell != domain.ShellFish {
		t.Errorf("got %+v, want shell=fish", got)
	}
}

func TestBuildGlobalAnswers_Invalid(t *testing.T) {
	if _, err := rules.BuildGlobalAnswers(rules.InitGlobalFlags{Shell: "powershell"}); !errors.Is(err, domain.ErrInvalidShellType) {
		t.Errorf("expected ErrInvalidShellType, got %v", err)
	}
}

func TestBuildProjectAnswers_FlagsWinOverDetection(t *testing.T) {
	detection := domain.InitDetectionResult{
		BaseBranch:     "develop",
		InstallCommand: "npm install",
		EnvFiles:       []domain.EnvFile{{Target: ".env", Template: ".env.example"}},
	}
	got, err := rules.BuildProjectAnswers(rules.InitProjectFlags{
		BasePath:       "../wt",
		BaseBranch:     "main",
		EnvStrategy:    "parent",
		InstallCommand: "pnpm install",
	}, detection)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.BasePath != "../wt" || got.BaseBranch != "main" {
		t.Errorf("flags did not win: %+v", got)
	}
	if got.EnvStrategy != domain.EnvStrategyParent {
		t.Errorf("flags did not win: %+v", got)
	}
	if len(got.OnCreate) == 0 || got.OnCreate[0].Cmd != "pnpm install" {
		t.Errorf("install flag should win in on_create: %+v", got.OnCreate)
	}
	if len(got.EnvFiles) != 1 || got.EnvFiles[0].Target != ".env" {
		t.Errorf("detected env files dropped: %+v", got.EnvFiles)
	}
}

func TestBuildProjectAnswers_FallsBackToDetectionThenDefaults(t *testing.T) {
	detection := domain.InitDetectionResult{BaseBranch: "trunk", InstallCommand: "go mod download"}
	got, err := rules.BuildProjectAnswers(rules.InitProjectFlags{}, detection)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.BasePath != domain.DefaultBasePath {
		t.Errorf("BasePath = %q, want default", got.BasePath)
	}
	if got.BaseBranch != "trunk" {
		t.Errorf("BaseBranch = %q, want detected trunk", got.BaseBranch)
	}
	if got.EnvStrategy != domain.DefaultEnvStrategy {
		t.Errorf("EnvStrategy = %q, want default", got.EnvStrategy)
	}
	if len(got.OnCreate) == 0 || got.OnCreate[0].Cmd != "go mod download" {
		t.Errorf("on_create = %+v, want detected install", got.OnCreate)
	}
}

func TestBuildProjectAnswers_NonInteractiveRequiresBaseBranch(t *testing.T) {
	_, err := rules.BuildProjectAnswers(rules.InitProjectFlags{NonInteractive: true}, domain.InitDetectionResult{})
	if err == nil {
		t.Fatal("expected error when base branch is unresolved in non-interactive mode")
	}
}

func TestBuildProjectAnswers_InvalidEnvStrategy(t *testing.T) {
	_, err := rules.BuildProjectAnswers(rules.InitProjectFlags{BaseBranch: "main", EnvStrategy: "nope"}, domain.InitDetectionResult{})
	if !errors.Is(err, domain.ErrInvalidEnvStrategy) {
		t.Errorf("expected ErrInvalidEnvStrategy, got %v", err)
	}
}

func TestBuildProjectAnswers_SkipEnv(t *testing.T) {
	detection := domain.InitDetectionResult{
		BaseBranch: "main",
		EnvFiles:   []domain.EnvFile{{Target: ".env"}, {Target: ".env.local", Local: true}},
	}
	got, err := rules.BuildProjectAnswers(rules.InitProjectFlags{SkipEnv: true}, detection)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.SkipEnv {
		t.Error("SkipEnv = false, want true")
	}
	if got.EnvStrategy != "" {
		t.Errorf("EnvStrategy = %q, want empty when skipped", got.EnvStrategy)
	}
	if len(got.EnvFiles) != 0 {
		t.Errorf("EnvFiles = %v, want none when skipped", got.EnvFiles)
	}
}

func TestBuildProjectAnswers_SkipEnvIgnoresInvalidStrategy(t *testing.T) {
	got, err := rules.BuildProjectAnswers(rules.InitProjectFlags{
		BaseBranch:  "main",
		EnvStrategy: "nope",
		SkipEnv:     true,
	}, domain.InitDetectionResult{})
	if err != nil {
		t.Fatalf("skipping env must bypass strategy validation, got %v", err)
	}
	if !got.SkipEnv {
		t.Error("SkipEnv = false, want true")
	}
}

func TestBuildProjectAnswers_SkipHooks(t *testing.T) {
	detection := domain.InitDetectionResult{
		BaseBranch:     "main",
		InstallCommand: "pnpm install",
		Monorepo: domain.MonorepoLayout{
			Kind:              domain.MonorepoKindWorkspace,
			WorkspacePackages: []string{"packages/a"},
		},
	}
	got, err := rules.BuildProjectAnswers(rules.InitProjectFlags{SkipHooks: true}, detection)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.SkipHooks {
		t.Error("SkipHooks = false, want true")
	}
	if len(got.OnCreate) != 0 {
		t.Errorf("OnCreate = %v, want none when skipped", got.OnCreate)
	}
}

// TestBuildProjectAnswers_NoServices asserts the global init no longer configures
// services: even when docker-compose files and scripts are detected, the answers
// carry no services — that surface belongs to `wtm run init` now.
func TestBuildProjectAnswers_NoServices(t *testing.T) {
	detection := domain.InitDetectionResult{
		BaseBranch:         "main",
		DockerComposeFiles: []string{"docker-compose.yml"},
		DockerComposeCmd:   "docker compose",
		PackageScripts:     []domain.PackageScript{{Name: "dev"}},
	}
	got, err := rules.BuildProjectAnswers(rules.InitProjectFlags{}, detection)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.DockerComposeFiles) != 0 || len(got.SelectedPackageScripts) != 0 {
		t.Errorf("global init should not populate services: docker=%v scripts=%v", got.DockerComposeFiles, got.SelectedPackageScripts)
	}
}

// TestAutoServicesAnswers verifies the non-interactive `wtm run init` path pulls
// every detected docker-compose file and package script into the answers.
func TestAutoServicesAnswers(t *testing.T) {
	detection := domain.InitDetectionResult{
		DockerComposeFiles: []string{"docker-compose.yml"},
		DockerComposeCmd:   "docker compose",
		PackageScripts:     []domain.PackageScript{{Name: "dev"}},
	}
	got := rules.AutoServicesAnswers(detection)
	if len(got.DockerComposeFiles) != 1 || got.DockerComposeCmd != "docker compose" {
		t.Errorf("docker not carried: files=%v cmd=%q", got.DockerComposeFiles, got.DockerComposeCmd)
	}
	if len(got.SelectedPackageScripts) != 1 {
		t.Errorf("scripts not carried: %v", got.SelectedPackageScripts)
	}
}

func TestInitProjectRecapFields_ResolvedValues(t *testing.T) {
	fields := rules.InitProjectRecapFields(domain.InitProjectAnswers{
		BasePath:    "../.trees",
		BaseBranch:  "main",
		EnvStrategy: domain.EnvStrategyExample,
		OnCreate:    []domain.HookCommand{{Cmd: "npm install", Cwd: "front"}, {Cmd: "pip install -r requirements.txt", Cwd: "api"}},
		OnClean:     []domain.HookCommand{{Cmd: "docker compose down"}},
	})
	got := map[string]string{}
	for _, f := range fields {
		got[f.Label] = f.Value
	}
	if got[domain.InitRecapLabelBasePath] != "../.trees" || got[domain.InitRecapLabelBaseBranch] != "main" {
		t.Errorf("base fields wrong: %+v", got)
	}
	if got[domain.InitRecapLabelEnvStrategy] != string(domain.EnvStrategyExample) {
		t.Errorf("env_strategy = %q, want %q", got[domain.InitRecapLabelEnvStrategy], domain.EnvStrategyExample)
	}
	if got[domain.InitRecapLabelOnCreate] != "npm install  (+1 more)" {
		t.Errorf("on_create = %q, want condensed +more form", got[domain.InitRecapLabelOnCreate])
	}
	if got[domain.InitRecapLabelOnClean] != "docker compose down" {
		t.Errorf("on_clean = %q, want the single hook command", got[domain.InitRecapLabelOnClean])
	}
}

func TestInitProjectRecapFields_Skipped(t *testing.T) {
	fields := rules.InitProjectRecapFields(domain.InitProjectAnswers{
		BasePath:   "../.trees",
		BaseBranch: "main",
		SkipEnv:    true,
		SkipHooks:  true,
	})
	got := map[string]string{}
	for _, f := range fields {
		got[f.Label] = f.Value
	}
	if got[domain.InitRecapLabelEnvStrategy] != domain.InitRecapValueSkippedTemplate {
		t.Errorf("env_strategy = %q, want %q", got[domain.InitRecapLabelEnvStrategy], domain.InitRecapValueSkippedTemplate)
	}
	if got[domain.InitRecapLabelOnCreate] != domain.InitRecapValueSkipped {
		t.Errorf("on_create = %q, want %q", got[domain.InitRecapLabelOnCreate], domain.InitRecapValueSkipped)
	}
}

// TestInitProjectRecapFields_OmitsEmptyOnCreate asserts the on_create row disappears
// (rather than showing a blank value) when no hook was configured and none skipped.
func TestInitProjectRecapFields_OmitsEmptyOnCreate(t *testing.T) {
	fields := rules.InitProjectRecapFields(domain.InitProjectAnswers{
		BasePath:    "../.trees",
		BaseBranch:  "main",
		EnvStrategy: domain.EnvStrategyExample,
	})
	for _, f := range fields {
		if f.Label == domain.InitRecapLabelOnCreate {
			t.Errorf("on_create row should be omitted when empty, got %q", f.Value)
		}
	}
}

func TestDisplayPath(t *testing.T) {
	if got := rules.DisplayPath(rules.DisplayPathParams{Base: "/repo", Target: "/repo/.git/wtm/config.toml"}); got != ".git/wtm/config.toml" {
		t.Errorf("DisplayPath inside base = %q, want relative", got)
	}
	// A target outside base keeps the absolute path rather than an ugly ../.. climb.
	if got := rules.DisplayPath(rules.DisplayPathParams{Base: "/repo", Target: "/elsewhere/wtm/config.toml"}); got != "/elsewhere/wtm/config.toml" {
		t.Errorf("DisplayPath outside base = %q, want unchanged absolute", got)
	}
}

// TestBuildProjectAnswers_WorkspaceMonorepoSeedsSingleHook: the root install of a
// workspace monorepo already installs every package, so no per-package hook is
// seeded.
func TestBuildProjectAnswers_WorkspaceMonorepoSeedsSingleHook(t *testing.T) {
	detection := domain.InitDetectionResult{
		BaseBranch:     "main",
		InstallCommand: "pnpm install",
		Monorepo: domain.MonorepoLayout{
			Kind:              domain.MonorepoKindWorkspace,
			WorkspacePackages: []string{"packages/a", "packages/b"},
		},
	}
	got, err := rules.BuildProjectAnswers(rules.InitProjectFlags{}, detection)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.OnCreate) != 1 {
		t.Fatalf("expected 1 on_create hook for a workspace, got %d: %+v", len(got.OnCreate), got.OnCreate)
	}
	if got.OnCreate[0].Cmd != "pnpm install" || got.OnCreate[0].Cwd != "" {
		t.Errorf("hook should be the bare install command: %+v", got.OnCreate[0])
	}
}

// TestBuildProjectAnswers_IndependentMonorepoFansOut: a repo with no workspace
// declaration but per-directory lockfiles gets one hook per directory, each with
// that directory's own package manager.
func TestBuildProjectAnswers_IndependentMonorepoFansOut(t *testing.T) {
	detection := domain.InitDetectionResult{
		BaseBranch: "main",
		Monorepo: domain.MonorepoLayout{
			Kind: domain.MonorepoKindIndependent,
			SubProjects: []domain.SubProject{
				{Dir: "api", PackageManager: domain.PkgManagerPip},
				{Dir: "front", PackageManager: domain.PkgManagerNpm},
			},
		},
	}
	got, err := rules.BuildProjectAnswers(rules.InitProjectFlags{}, detection)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []domain.HookCommand{
		{Cmd: domain.InstallCommandPip, Cwd: "api"},
		{Cmd: domain.InstallCommandNpm, Cwd: "front"},
	}
	if len(got.OnCreate) != len(want) {
		t.Fatalf("got %d hooks, want %d: %+v", len(got.OnCreate), len(want), got.OnCreate)
	}
	for i := range want {
		if got.OnCreate[i] != want[i] {
			t.Errorf("hook[%d] = %+v, want %+v", i, got.OnCreate[i], want[i])
		}
	}
}
