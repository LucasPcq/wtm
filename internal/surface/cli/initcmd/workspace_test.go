package initcmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/detect"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A declared workspace is installed from its root by every package manager:
// one install there links every package, and a second one per package is at
// best redundant and at worst a nested lockfile.
func TestInitInstallsAWorkspaceOnceFromItsRoot(t *testing.T) {
	tests := []struct {
		name     string
		lockfile string
		declare  func(t *testing.T, dir string)
		install  string
	}{
		{name: "npm", lockfile: "package-lock.json", install: "npm install", declare: manifestWorkspaces},
		{name: "yarn", lockfile: "yarn.lock", install: "yarn install", declare: manifestWorkspaces},
		{name: "pnpm", lockfile: "pnpm-lock.yaml", install: "pnpm install", declare: func(t *testing.T, dir string) {
			writeFile(t, filepath.Join(dir, "package.json"), `{"name":"root"}`)
			writeFile(t, filepath.Join(dir, "pnpm-workspace.yaml"), "packages:\n  - \"packages/*\"\n")
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.declare(t, dir)
			writeFile(t, filepath.Join(dir, tt.lockfile), "")
			writeFile(t, filepath.Join(dir, "packages", "api", "package.json"), `{"name":"api"}`)
			writeFile(t, filepath.Join(dir, "packages", "web", "package.json"), `{"name":"web"}`)

			answers, err := rules.BuildProjectAnswers(rules.InitProjectFlags{BaseBranch: "main"}, detect.ProjectEnvironment(t.Context(), dir))
			if err != nil {
				t.Fatalf("BuildProjectAnswers: %v", err)
			}
			want := []domain.HookCommand{{Cmd: tt.install}}
			if len(answers.OnCreate) != 1 || answers.OnCreate[0] != want[0] {
				t.Errorf("on_create = %+v, want %+v", answers.OnCreate, want)
			}
		})
	}
}

func manifestWorkspaces(t *testing.T, dir string) {
	writeFile(t, filepath.Join(dir, "package.json"), `{"name":"root","workspaces":["packages/*"]}`)
}

// `init --only hooks` with nobody to ask keeps the hooks the config holds: the
// first of them is not an install command to copy into every package.
func TestReinitHooksKeepsTheConfiguredList(t *testing.T) {
	dir := t.TempDir()
	cfg := fullSeededConfig()
	cfg.Hooks.OnCreate = []domain.HookCommand{
		{Cmd: "make bootstrap"},
		{Cmd: "cp .env.example .env", Cwd: "apps/api"},
	}
	seedProjectConfig(t, dir, cfg)

	detection := domain.InitDetectionResult{InstallCommand: "pnpm install"}
	got, err := buildReinitAnswers(newReinitTestCmd(), dir, detection)
	if err != nil {
		t.Fatalf("buildReinitAnswers: %v", err)
	}
	if len(got.OnCreate) != 2 || got.OnCreate[0] != cfg.Hooks.OnCreate[0] || got.OnCreate[1] != cfg.Hooks.OnCreate[1] {
		t.Errorf("on_create = %+v, want %+v", got.OnCreate, cfg.Hooks.OnCreate)
	}
}
