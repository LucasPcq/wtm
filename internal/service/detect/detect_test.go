package detect

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func touchFile(t *testing.T, path string) {
	t.Helper()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPackageManagerPnpm(t *testing.T) {
	dir := t.TempDir()
	touchFile(t, filepath.Join(dir, domain.LockfilePnpm))

	pm := PackageManager(dir)
	if pm != domain.PkgManagerPnpm {
		t.Errorf("expected pnpm, got %s", pm)
	}
}

func TestPackageManagerNpm(t *testing.T) {
	dir := t.TempDir()
	touchFile(t, filepath.Join(dir, domain.LockfileNpm))

	pm := PackageManager(dir)
	if pm != domain.PkgManagerNpm {
		t.Errorf("expected npm, got %s", pm)
	}
}

func TestPackageManagerYarn(t *testing.T) {
	dir := t.TempDir()
	touchFile(t, filepath.Join(dir, domain.LockfileYarn))

	pm := PackageManager(dir)
	if pm != domain.PkgManagerYarn {
		t.Errorf("expected yarn, got %s", pm)
	}
}

func TestPackageManagerGo(t *testing.T) {
	dir := t.TempDir()
	touchFile(t, filepath.Join(dir, domain.LockfileGo))

	pm := PackageManager(dir)
	if pm != domain.PkgManagerGo {
		t.Errorf("expected go, got %s", pm)
	}
}

func TestPackageManagerPip(t *testing.T) {
	dir := t.TempDir()
	touchFile(t, filepath.Join(dir, domain.LockfilePip))

	pm := PackageManager(dir)
	if pm != domain.PkgManagerPip {
		t.Errorf("expected pip, got %s", pm)
	}
}

func TestPackageManagerNone(t *testing.T) {
	dir := t.TempDir()

	pm := PackageManager(dir)
	if pm != domain.PkgManagerNone {
		t.Errorf("expected none, got %s", pm)
	}
}

func TestPackageManagerPriorityOrder(t *testing.T) {
	dir := t.TempDir()
	touchFile(t, filepath.Join(dir, domain.LockfilePnpm))
	touchFile(t, filepath.Join(dir, domain.LockfileNpm))

	pm := PackageManager(dir)
	if pm != domain.PkgManagerPnpm {
		t.Errorf("expected pnpm (higher priority), got %s", pm)
	}
}

func TestInstallCommand(t *testing.T) {
	tests := []struct {
		pm   domain.PackageManager
		want string
	}{
		{domain.PkgManagerPnpm, domain.InstallCommandPnpm},
		{domain.PkgManagerNpm, domain.InstallCommandNpm},
		{domain.PkgManagerYarn, domain.InstallCommandYarn},
		{domain.PkgManagerGo, domain.InstallCommandGo},
		{domain.PkgManagerPip, domain.InstallCommandPip},
		{domain.PkgManagerNone, ""},
	}

	for _, tt := range tests {
		got := rules.InstallCommand(tt.pm)
		if got != tt.want {
			t.Errorf("InstallCommand(%s) = %q, want %q", tt.pm, got, tt.want)
		}
	}
}

func envFileByTarget(files []domain.EnvFile, target string) (domain.EnvFile, bool) {
	for _, f := range files {
		if f.Target == target {
			return f, true
		}
	}
	return domain.EnvFile{}, false
}

func TestEnvFilesPairsTemplateWithTarget(t *testing.T) {
	dir := t.TempDir()
	touchFile(t, filepath.Join(dir, ".env"))
	touchFile(t, filepath.Join(dir, ".env.example"))
	touchFile(t, filepath.Join(dir, "apps", "api", ".env"))
	touchFile(t, filepath.Join(dir, "apps", "api", ".env.example"))

	files := EnvFiles(dir)
	if len(files) != 2 {
		t.Fatalf("expected 2 targets (.env, apps/api/.env), got %d: %v", len(files), files)
	}
	root, ok := envFileByTarget(files, ".env")
	if !ok || root.Template != ".env.example" {
		t.Errorf("root .env not paired with .env.example: %+v", files)
	}
	nested, ok := envFileByTarget(files, filepath.Join("apps", "api", ".env"))
	if !ok || nested.Template != filepath.Join("apps", "api", ".env.example") {
		t.Errorf("nested .env not paired with its template: %+v", files)
	}
}

func TestEnvFilesLoneTemplateYieldsTarget(t *testing.T) {
	dir := t.TempDir()
	touchFile(t, filepath.Join(dir, ".env.dist"))

	files := EnvFiles(dir)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d: %v", len(files), files)
	}
	if files[0].Target != ".env" || files[0].Template != ".env.dist" {
		t.Errorf("expected target .env with template .env.dist, got %+v", files[0])
	}
}

func TestEnvFilesTemplatePriority(t *testing.T) {
	dir := t.TempDir()
	touchFile(t, filepath.Join(dir, ".env.dist"))
	touchFile(t, filepath.Join(dir, ".env.example"))

	files := EnvFiles(dir)
	if len(files) != 1 || files[0].Template != ".env.example" {
		t.Errorf("expected .env.example pinned over .env.dist, got %+v", files)
	}
}

func TestEnvFilesLocalFlagged(t *testing.T) {
	dir := t.TempDir()
	touchFile(t, filepath.Join(dir, ".env"))
	touchFile(t, filepath.Join(dir, ".env.local"))

	files := EnvFiles(dir)
	local, ok := envFileByTarget(files, ".env.local")
	if !ok || !local.Local {
		t.Errorf("expected .env.local detected and flagged local, got %+v", files)
	}
}

func TestEnvFilesIgnoresMultiEnv(t *testing.T) {
	dir := t.TempDir()
	touchFile(t, filepath.Join(dir, ".env.production"))
	touchFile(t, filepath.Join(dir, ".env.staging"))

	files := EnvFiles(dir)
	if len(files) != 0 {
		t.Fatalf("multi-env files should be ignored, got %+v", files)
	}
}

func TestEnvFilesExcludesNodeModules(t *testing.T) {
	dir := t.TempDir()
	touchFile(t, filepath.Join(dir, ".env"))
	touchFile(t, filepath.Join(dir, "node_modules", ".env"))

	files := EnvFiles(dir)
	if len(files) != 1 {
		t.Fatalf("expected 1 env file (excluded node_modules), got %d: %v", len(files), files)
	}
}

func TestEnvFilesEmptyDir(t *testing.T) {
	dir := t.TempDir()

	files := EnvFiles(dir)
	if len(files) != 0 {
		t.Fatalf("expected 0 env files, got %d: %v", len(files), files)
	}
}

func TestProjectConfigExists(t *testing.T) {
	dir := t.TempDir()
	if ProjectConfigExists(dir) {
		t.Error("expected false for empty dir")
	}

	touchFile(t, filepath.Join(dir, domain.ConfigFileName))
	if !ProjectConfigExists(dir) {
		t.Error("expected true after creating config.toml")
	}
}

func TestBaseBranchFallback(t *testing.T) {
	dir := t.TempDir()

	branch := BaseBranch(dir)
	if branch != domain.DefaultBaseBranch {
		t.Errorf("expected fallback %s, got %s", domain.DefaultBaseBranch, branch)
	}
}

func TestDockerComposeFilesNone(t *testing.T) {
	dir := t.TempDir()
	files := DockerComposeFiles(dir)
	if len(files) != 0 {
		t.Errorf("expected 0, got %d: %v", len(files), files)
	}
}

func TestDockerComposeFilesSingle(t *testing.T) {
	dir := t.TempDir()
	touchFile(t, filepath.Join(dir, "docker-compose.yml"))

	files := DockerComposeFiles(dir)
	if len(files) != 1 || files[0] != "docker-compose.yml" {
		t.Errorf("expected [docker-compose.yml], got %v", files)
	}
}

func TestDockerComposeFilesMultiple(t *testing.T) {
	dir := t.TempDir()
	touchFile(t, filepath.Join(dir, "docker-compose.yml"))
	touchFile(t, filepath.Join(dir, "docker-compose.dev.yml"))
	touchFile(t, filepath.Join(dir, "docker-compose.prod.yaml"))

	files := DockerComposeFiles(dir)
	if len(files) != 3 {
		t.Errorf("expected 3, got %d: %v", len(files), files)
	}
}

func TestWorkspacePackagesNone(t *testing.T) {
	dir := t.TempDir()
	pkgs := WorkspacePackages(dir)
	if pkgs != nil {
		t.Errorf("expected nil, got %v", pkgs)
	}
}

func TestWorkspacePackages(t *testing.T) {
	dir := t.TempDir()

	ws := "packages:\n  - \"apps/*\"\n  - \"packages/*\"\n"
	if err := os.WriteFile(filepath.Join(dir, "pnpm-workspace.yaml"), []byte(ws), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, pkg := range []string{"apps/web", "apps/api", "packages/shared"} {
		writeManifest(t, dir, pkg, `{"name":"`+filepath.Base(pkg)+`"}`)
	}

	pkgs := WorkspacePackages(dir)
	if len(pkgs) != 3 {
		t.Fatalf("expected 3 packages, got %d: %v", len(pkgs), pkgs)
	}
}

// A globstar selects any depth: Go's filepath.Glob stopped at the first level,
// which hid apps/app-1/back from detection entirely (LUC-208).
func TestWorkspacePackagesGlobstarReachesDeepPackages(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pnpm-workspace.yaml"), []byte("packages:\n  - \"apps/**\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeManifest(t, dir, "apps/app-1/back", `{"name":"back"}`)
	writeManifest(t, dir, "apps/app-1/front", `{"name":"front"}`)
	writeManifest(t, dir, "node_modules/junk", `{"name":"junk"}`)

	pkgs := WorkspacePackages(dir)
	want := []string{"apps/app-1/back", "apps/app-1/front"}
	if len(pkgs) != len(want) {
		t.Fatalf("expected %v, got %v", want, pkgs)
	}
	for i, pkg := range want {
		if pkgs[i] != pkg {
			t.Errorf("expected %s at %d, got %s", pkg, i, pkgs[i])
		}
	}
}

// npm, yarn and bun declare their workspaces in the root manifest instead.
func TestWorkspacePackagesFromRootManifest(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, ".", `{"name":"root","workspaces":["apps/*/*","!apps/app-1/front"]}`)
	writeManifest(t, dir, "apps/app-1/back", `{"name":"back"}`)
	writeManifest(t, dir, "apps/app-1/front", `{"name":"front"}`)

	pkgs := WorkspacePackages(dir)
	if len(pkgs) != 1 || pkgs[0] != "apps/app-1/back" {
		t.Fatalf("expected [apps/app-1/back], got %v", pkgs)
	}
}

func writeManifest(t *testing.T, dir, rel, content string) {
	t.Helper()
	target := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "package.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestParsePnpmWorkspace(t *testing.T) {
	content := "packages:\n  - 'apps/*'\n  - \"packages/*\"\n  - '!tests'\n"
	patterns := parsePnpmWorkspace(content)

	// Negations are kept and applied by rules.SelectsWorkspace: dropping them
	// here would widen the selection they exist to narrow.
	if len(patterns) != 3 {
		t.Fatalf("expected 3 patterns, got %d: %v", len(patterns), patterns)
	}
	if patterns[0] != "apps/*" {
		t.Errorf("expected apps/*, got %s", patterns[0])
	}
	if patterns[1] != "packages/*" {
		t.Errorf("expected packages/*, got %s", patterns[1])
	}
}
