package detect

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
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

func TestWorkspaceDeclaration(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		want  bool
	}{
		{name: "bare dir", files: map[string]string{}, want: false},
		{name: "pnpm-workspace.yaml", files: map[string]string{"pnpm-workspace.yaml": "packages:\n  - 'apps/*'\n"}, want: true},
		{name: "go.work", files: map[string]string{"go.work": "go 1.26\n"}, want: true},
		{name: "package.json workspaces array", files: map[string]string{"package.json": `{"workspaces":["apps/*"]}`}, want: true},
		{name: "package.json workspaces yarn v1 object", files: map[string]string{"package.json": `{"workspaces":{"packages":["apps/*"]}}`}, want: true},
		{name: "turbo.json", files: map[string]string{"turbo.json": "{}"}, want: true},
		{name: "nx.json", files: map[string]string{"nx.json": "{}"}, want: true},
		{name: "lerna.json", files: map[string]string{"lerna.json": "{}"}, want: true},
		// An empty workspaces array is not a workspace — it must not suppress the
		// per-directory fan-out for a repo that genuinely needs it.
		{name: "package.json empty workspaces", files: map[string]string{"package.json": `{"workspaces":[]}`}, want: false},
		{name: "package.json without workspaces", files: map[string]string{"package.json": `{"name":"app"}`}, want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range tc.files {
				writeFile(t, filepath.Join(dir, name), content)
			}
			if got := WorkspaceDeclaration(dir); got != tc.want {
				t.Errorf("WorkspaceDeclaration = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestWorkspacePackagesFiltersDirsWithoutPackageJSON is the guard against seeding
// hooks for globbed directories that are not packages at all.
func TestWorkspacePackagesFiltersDirsWithoutPackageJSON(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pnpm-workspace.yaml"), "packages:\n  - \"apps/*\"\n")
	writeFile(t, filepath.Join(dir, "apps", "web", "package.json"), `{"name":"web"}`)
	writeFile(t, filepath.Join(dir, "apps", "api", "package.json"), `{"name":"api"}`)
	if err := os.MkdirAll(filepath.Join(dir, "apps", "fixtures"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := WorkspacePackages(dir)
	want := []string{"apps/api", "apps/web"}
	if len(got) != len(want) {
		t.Fatalf("WorkspacePackages = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("WorkspacePackages[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestWorkspacePackagesFromPackageJSONWorkspaces(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), `{"name":"root","workspaces":["apps/*"]}`)
	writeFile(t, filepath.Join(dir, "apps", "web", "package.json"), `{"name":"web"}`)

	got := WorkspacePackages(dir)
	if len(got) != 1 || got[0] != "apps/web" {
		t.Errorf("WorkspacePackages = %v, want [apps/web]", got)
	}
}

// TestWorkspacePackagesGlobstar covers the "**" pattern filepath.Glob cannot
// expand on its own.
func TestWorkspacePackagesGlobstar(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pnpm-workspace.yaml"), "packages:\n  - \"packages/**\"\n")
	writeFile(t, filepath.Join(dir, "packages", "group", "pkg", "package.json"), `{"name":"pkg"}`)
	writeFile(t, filepath.Join(dir, "packages", "flat", "package.json"), `{"name":"flat"}`)
	writeFile(t, filepath.Join(dir, "packages", "flat", "node_modules", "dep", "package.json"), `{"name":"dep"}`)

	got := WorkspacePackages(dir)
	want := []string{"packages/flat", "packages/group/pkg"}
	if len(got) != len(want) {
		t.Fatalf("WorkspacePackages = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("WorkspacePackages[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestWorkspacePackagesNoDeclaration(t *testing.T) {
	if got := WorkspacePackages(t.TempDir()); got != nil {
		t.Errorf("WorkspacePackages = %v, want nil", got)
	}
}

// TestMonorepoWorkspaceWinsOverSubLockfiles is the regression guard for the
// reported bug: a workspace root must never fan out, even when its packages
// carry lockfiles of their own.
func TestMonorepoWorkspaceWinsOverSubLockfiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pnpm-workspace.yaml"), "packages:\n  - \"apps/*\"\n")
	writeFile(t, filepath.Join(dir, "pnpm-lock.yaml"), "")
	writeFile(t, filepath.Join(dir, "apps", "web", "package.json"), `{"name":"web"}`)
	writeFile(t, filepath.Join(dir, "apps", "web", "package-lock.json"), "{}")

	layout := Monorepo(dir)
	if layout.Kind != domain.MonorepoKindWorkspace {
		t.Fatalf("Kind = %q, want %q", layout.Kind, domain.MonorepoKindWorkspace)
	}
	if len(layout.SubProjects) != 0 {
		t.Errorf("SubProjects = %v, want empty for a workspace", layout.SubProjects)
	}
	if len(layout.WorkspacePackages) != 1 || layout.WorkspacePackages[0] != "apps/web" {
		t.Errorf("WorkspacePackages = %v, want [apps/web]", layout.WorkspacePackages)
	}
}

func TestMonorepoIndependent(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "front", "package-lock.json"), "{}")
	writeFile(t, filepath.Join(dir, "api", "requirements.txt"), "")

	layout := Monorepo(dir)
	if layout.Kind != domain.MonorepoKindIndependent {
		t.Fatalf("Kind = %q, want %q", layout.Kind, domain.MonorepoKindIndependent)
	}
	if len(layout.SubProjects) != 2 {
		t.Fatalf("SubProjects = %v, want 2", layout.SubProjects)
	}
}

func TestMonorepoNone(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package-lock.json"), "{}")

	layout := Monorepo(dir)
	if layout.Kind != domain.MonorepoKindNone {
		t.Errorf("Kind = %q, want %q", layout.Kind, domain.MonorepoKindNone)
	}
}
