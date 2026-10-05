package infra

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

func TestIsDirty_CleanRepo(t *testing.T) {
	dir := gittest.InitRepo(t)

	dirty, err := IsDirty(t.Context(), IsDirtyParams{WorktreePath: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dirty {
		t.Error("expected clean repo to not be dirty")
	}
}

func TestIsDirty_DirtyRepo(t *testing.T) {
	dir := gittest.InitRepo(t)

	os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("hello"), 0o644)

	dirty, err := IsDirty(t.Context(), IsDirtyParams{WorktreePath: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !dirty {
		t.Error("expected repo with untracked file to be dirty")
	}
}

func TestListModifiedFiles_ExplodesUntrackedDirectories(t *testing.T) {
	dir := gittest.InitRepo(t)
	writeStatusFile(t, dir, "newmod/x.go", "package newmod")
	writeStatusFile(t, dir, "newmod/sub/y.go", "package sub")

	paths := listedPaths(t, dir)

	if _, collapsed := paths["newmod/"]; collapsed {
		t.Fatal("untracked directory reported as a single entry, files cannot be picked apart")
	}
	for _, want := range []string{"newmod/x.go", "newmod/sub/y.go"} {
		if _, ok := paths[want]; !ok {
			t.Errorf("missing %q in %v", want, paths)
		}
	}
}

func TestListModifiedFiles_KeepsPathsVerbatim(t *testing.T) {
	dir := gittest.InitRepo(t)
	// git quotes these under the default porcelain format; -z must not.
	writeStatusFile(t, dir, "a b.txt", "spaces")
	writeStatusFile(t, dir, "café.txt", "non-ascii")

	paths := listedPaths(t, dir)

	for _, want := range []string{"a b.txt", "café.txt"} {
		if _, ok := paths[want]; !ok {
			t.Errorf("missing %q in %v", want, paths)
		}
	}
}

func TestListModifiedFiles_ReportsRenameOrigin(t *testing.T) {
	dir := gittest.InitRepo(t)
	writeStatusFile(t, dir, "old.txt", "content")
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-m", "add old")
	gitRun(t, dir, "mv", "old.txt", "new.txt")

	entries, err := ListModifiedFiles(t.Context(), ListModifiedFilesParams{WorktreePath: dir})
	if err != nil {
		t.Fatalf("ListModifiedFiles: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %+v, want a single rename entry", entries)
	}
	if entries[0].Path != "new.txt" || entries[0].OrigPath != "old.txt" {
		t.Fatalf("got %+v, want new.txt from old.txt", entries[0])
	}
}

func TestListModifiedFiles_SkipsIgnoredFiles(t *testing.T) {
	dir := gittest.InitRepo(t)
	writeStatusFile(t, dir, ".gitignore", "secret.env\n")
	writeStatusFile(t, dir, "secret.env", "TOKEN=x")

	paths := listedPaths(t, dir)

	if _, ok := paths["secret.env"]; ok {
		t.Error("gitignored file must not be offered for extraction")
	}
}

func listedPaths(t *testing.T, dir string) map[string]string {
	t.Helper()
	entries, err := ListModifiedFiles(t.Context(), ListModifiedFilesParams{WorktreePath: dir})
	if err != nil {
		t.Fatalf("ListModifiedFiles: %v", err)
	}

	paths := make(map[string]string, len(entries))
	for _, e := range entries {
		paths[e.Path] = e.Status
	}
	return paths
}

func writeStatusFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", rel, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %s: %v", args, out, err)
	}
}
