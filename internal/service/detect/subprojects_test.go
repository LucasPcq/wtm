package detect

import (
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

// TestSubProjectsPerDirectoryPackageManager proves each sub-project reports its
// OWN package manager rather than inheriting the root's.
func TestSubProjectsPerDirectoryPackageManager(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "front", "package-lock.json"), "{}")
	writeFile(t, filepath.Join(dir, "api", "requirements.txt"), "")

	got := SubProjects(SubProjectsParams{Root: dir, MaxDepth: domain.MonorepoScanMaxDepth})
	want := []domain.SubProject{
		{Dir: "api", PackageManager: domain.PkgManagerPip},
		{Dir: "front", PackageManager: domain.PkgManagerNpm},
	}
	if len(got) != len(want) {
		t.Fatalf("SubProjects = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("SubProjects[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestSubProjectsSkipsIgnoredDirs(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "node_modules", "dep", "package-lock.json"), "{}")
	writeFile(t, filepath.Join(dir, ".trees", "feature", "go.mod"), "module x\n")
	writeFile(t, filepath.Join(dir, "vendor", "lib", "go.mod"), "module y\n")

	if got := SubProjects(SubProjectsParams{Root: dir, MaxDepth: domain.MonorepoScanMaxDepth}); len(got) != 0 {
		t.Errorf("SubProjects = %v, want none", got)
	}
}

// TestSubProjectsDoesNotDescendIntoMatch: the outermost lockfile owns the
// directory, so a nested module does not add a second install hook.
func TestSubProjectsDoesNotDescendIntoMatch(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "front", "package-lock.json"), "{}")
	writeFile(t, filepath.Join(dir, "front", "sub", "go.mod"), "module z\n")

	got := SubProjects(SubProjectsParams{Root: dir, MaxDepth: domain.MonorepoScanMaxDepth})
	if len(got) != 1 || got[0].Dir != "front" {
		t.Errorf("SubProjects = %v, want [front]", got)
	}
}

func TestSubProjectsRespectsMaxDepth(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a", "b", "c", "d", "go.mod"), "module deep\n")

	if got := SubProjects(SubProjectsParams{Root: dir, MaxDepth: 3}); len(got) != 0 {
		t.Errorf("SubProjects = %v, want none beyond MaxDepth", got)
	}
}

// TestSubProjectsExcludesRoot: a plain single project has no sub-projects, so it
// classifies as MonorepoKindNone and keeps its single root install.
func TestSubProjectsExcludesRoot(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module root\n")

	if got := SubProjects(SubProjectsParams{Root: dir, MaxDepth: domain.MonorepoScanMaxDepth}); len(got) != 0 {
		t.Errorf("SubProjects = %v, want none", got)
	}
}
