package infra

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

func gitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %s: %v", strings.Join(args, " "), out, err)
	}
}

func writeCommit(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	gitCmd(t, dir, "add", name)
	gitCmd(t, dir, "commit", "-m", "commit "+name)
}

// TestConflictedFiles_ReportsUnmergedPaths verifies ConflictedFiles lists the
// files with conflict markers during a stopped merge, and nothing on a clean tree.
func TestConflictedFiles_ReportsUnmergedPaths(t *testing.T) {
	dir := gittest.InitRepo(t)

	if got := ConflictedFiles(t.Context(), dir); got != nil {
		t.Fatalf("clean tree: ConflictedFiles = %v, want nil", got)
	}

	writeCommit(t, dir, "a.txt", "base")

	gitCmd(t, dir, "checkout", "-b", "feature")
	writeCommit(t, dir, "a.txt", "feature change")

	gitCmd(t, dir, "checkout", "main")
	writeCommit(t, dir, "a.txt", "main change")

	// A conflicting merge stops with a.txt unmerged (non-zero exit expected).
	exec.Command("git", "-C", dir, "merge", "feature").Run()

	files := ConflictedFiles(t.Context(), dir)
	if len(files) != 1 || files[0] != "a.txt" {
		t.Fatalf("ConflictedFiles = %v, want [a.txt]", files)
	}
}
