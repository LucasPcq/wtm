package detect

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func writePackageJSON(t *testing.T, dir, name string, scripts map[string]string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(map[string]any{"name": name, "scripts": scripts})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPackageJSONScriptsNoFile(t *testing.T) {
	dir := t.TempDir()
	if got := PackageJSONScripts(dir); got != nil {
		t.Errorf("expected nil for missing package.json, got %v", got)
	}
}

func TestPackageJSONScriptsRoot(t *testing.T) {
	dir := t.TempDir()
	writePackageJSON(t, dir, "my-app", map[string]string{
		"dev":   "vite",
		"build": "vite build",
		"test":  "vitest",
	})

	got := PackageJSONScripts(dir)
	if len(got) != 3 {
		t.Fatalf("expected 3 scripts, got %d: %v", len(got), got)
	}

	// Alphabetical order.
	assertScript(t, got[0], domain.PackageScript{Name: "build", Cmd: "vite build", Workspace: "", PkgName: "my-app"})
	assertScript(t, got[1], domain.PackageScript{Name: "dev", Cmd: "vite", Workspace: "", PkgName: "my-app"})
	assertScript(t, got[2], domain.PackageScript{Name: "test", Cmd: "vitest", Workspace: "", PkgName: "my-app"})
}

func TestPackageJSONScriptsMonorepo(t *testing.T) {
	dir := t.TempDir()

	writePackageJSON(t, dir, "root", map[string]string{"dev": "turbo dev"})

	if err := os.WriteFile(filepath.Join(dir, "pnpm-workspace.yaml"), []byte("packages:\n  - \"apps/*\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	writePackageJSON(t, filepath.Join(dir, "apps", "web"), "@demo/web", map[string]string{
		"dev":   "next dev",
		"build": "next build",
	})
	writePackageJSON(t, filepath.Join(dir, "apps", "api"), "@demo/api", map[string]string{
		"dev": "tsx watch src",
	})

	// A globbed directory that is not a package must not reach script discovery.
	if err := os.MkdirAll(filepath.Join(dir, "apps", "fixtures"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := PackageJSONScripts(dir)
	// root: 1, apps/api: 1, apps/web: 2 = 4 total
	if len(got) != 4 {
		t.Fatalf("expected 4 scripts, got %d: %v", len(got), got)
	}

	// root first
	assertScript(t, got[0], domain.PackageScript{Name: "dev", Cmd: "turbo dev", Workspace: "", PkgName: "root"})
	// workspace apps/api (alphabetically before apps/web)
	assertScript(t, got[1], domain.PackageScript{Name: "dev", Cmd: "tsx watch src", Workspace: "apps/api", PkgName: "api"})
	// workspace apps/web, alphabetical within package
	assertScript(t, got[2], domain.PackageScript{Name: "build", Cmd: "next build", Workspace: "apps/web", PkgName: "web"})
	assertScript(t, got[3], domain.PackageScript{Name: "dev", Cmd: "next dev", Workspace: "apps/web", PkgName: "web"})
}

func TestPackageJSONScriptsScopeStripping(t *testing.T) {
	dir := t.TempDir()
	writePackageJSON(t, dir, "@acme/backend", map[string]string{"start": "node dist/index.js"})

	got := PackageJSONScripts(dir)
	if len(got) != 1 {
		t.Fatalf("expected 1 script, got %d", len(got))
	}
	if got[0].PkgName != "backend" {
		t.Errorf("expected PkgName=backend, got %q", got[0].PkgName)
	}
}

func assertScript(t *testing.T, got, want domain.PackageScript) {
	t.Helper()
	if got.Name != want.Name || got.Cmd != want.Cmd || got.Workspace != want.Workspace || got.PkgName != want.PkgName {
		t.Errorf("got %+v, want %+v", got, want)
	}
}
