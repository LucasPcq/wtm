package worktree

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

// extractEnv holds the two worktrees used by the extract tests.
type extractEnv struct {
	source string
	target string
}

// setupExtract builds a repo with files a.txt and b.txt committed, plus a target
// worktree branched from the same HEAD.
func setupExtract(t *testing.T) extractEnv {
	t.Helper()
	source := gittest.InitRepo(t)
	writeFile(t, source, "a.txt", "line1\nline2\nline3\n")
	writeFile(t, source, "b.txt", "base\n")
	gitRun(t, source, "add", ".")
	gitRun(t, source, "commit", "-qm", "files")

	target := filepath.Join(t.TempDir(), "tgt")
	gitRun(t, source, "worktree", "add", "-q", "-b", "feat", target, "HEAD")

	return extractEnv{source: source, target: target}
}

func TestExtractMovesModifiedFile(t *testing.T) {
	env := setupExtract(t)
	writeFile(t, env.source, "a.txt", "line1\nCHANGED\nline3\n")

	result, err := Extract(t.Context(), domain.ExtractParams{
		SourcePath: env.source,
		TargetPath: env.target,
		Files:      []domain.ExtractFile{{Path: "a.txt", Status: domain.ExtractStatusModified}},
	})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	if got := readFile(t, env.target, "a.txt"); got != "line1\nCHANGED\nline3\n" {
		t.Errorf("target a.txt = %q, want changed content", got)
	}
	if got := readFile(t, env.source, "a.txt"); got != "line1\nline2\nline3\n" {
		t.Errorf("source a.txt = %q, want restored to HEAD", got)
	}
	if status := gitStatus(t, env.source); status != "" {
		t.Errorf("source not clean after move: %q", status)
	}
	if len(result.Files) != 1 || result.Files[0].Path != "a.txt" {
		t.Errorf("result.Files = %v", result.Files)
	}
}

func TestExtractMovesUntrackedFile(t *testing.T) {
	env := setupExtract(t)
	writeFile(t, env.source, "c.txt", "brand new\n")

	_, err := Extract(t.Context(), domain.ExtractParams{
		SourcePath: env.source,
		TargetPath: env.target,
		Files:      []domain.ExtractFile{{Path: "c.txt", Status: domain.ExtractStatusUntracked}},
	})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	if got := readFile(t, env.target, "c.txt"); got != "brand new\n" {
		t.Errorf("target c.txt = %q", got)
	}
	if fileExists(env.source, "c.txt") {
		t.Error("source c.txt should be removed after move")
	}
}

func TestExtractMovesDeletedFile(t *testing.T) {
	env := setupExtract(t)
	if err := os.Remove(filepath.Join(env.source, "b.txt")); err != nil {
		t.Fatal(err)
	}

	_, err := Extract(t.Context(), domain.ExtractParams{
		SourcePath: env.source,
		TargetPath: env.target,
		Files:      []domain.ExtractFile{{Path: "b.txt", Status: domain.ExtractStatusDeleted}},
	})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	if fileExists(env.target, "b.txt") {
		t.Error("target b.txt should be deleted")
	}
	if !fileExists(env.source, "b.txt") {
		t.Error("source b.txt should be restored to HEAD")
	}
}

func TestExtractKeepLeavesSource(t *testing.T) {
	env := setupExtract(t)
	writeFile(t, env.source, "a.txt", "line1\nKEEP\nline3\n")

	_, err := Extract(t.Context(), domain.ExtractParams{
		SourcePath: env.source,
		TargetPath: env.target,
		Keep:       true,
		Files:      []domain.ExtractFile{{Path: "a.txt", Status: domain.ExtractStatusModified}},
	})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	if got := readFile(t, env.source, "a.txt"); got != "line1\nKEEP\nline3\n" {
		t.Errorf("source a.txt = %q, want kept", got)
	}
	if got := readFile(t, env.target, "a.txt"); got != "line1\nKEEP\nline3\n" {
		t.Errorf("target a.txt = %q, want copied", got)
	}
}

func TestExtractConflictAbortsAndLeavesSourceIntact(t *testing.T) {
	env := setupExtract(t)
	writeFile(t, env.source, "a.txt", "line1\nSRC\nline3\n")
	writeFile(t, env.target, "a.txt", "line1\nTGT\nline3\n")

	_, err := Extract(t.Context(), domain.ExtractParams{
		SourcePath: env.source,
		TargetPath: env.target,
		Files:      []domain.ExtractFile{{Path: "a.txt", Status: domain.ExtractStatusModified}},
	})
	if !errors.Is(err, domain.ErrExtractConflict) {
		t.Fatalf("err = %v, want ErrExtractConflict", err)
	}

	if got := readFile(t, env.source, "a.txt"); got != "line1\nSRC\nline3\n" {
		t.Errorf("source a.txt changed on conflict: %q", got)
	}
	if got := readFile(t, env.target, "a.txt"); got != "line1\nTGT\nline3\n" {
		t.Errorf("target a.txt changed on conflict: %q", got)
	}
}

func TestExtractUntrackedCollisionAborts(t *testing.T) {
	env := setupExtract(t)
	writeFile(t, env.source, "c.txt", "from source\n")
	writeFile(t, env.target, "c.txt", "already here\n")

	_, err := Extract(t.Context(), domain.ExtractParams{
		SourcePath: env.source,
		TargetPath: env.target,
		Files:      []domain.ExtractFile{{Path: "c.txt", Status: domain.ExtractStatusUntracked}},
	})
	if !errors.Is(err, domain.ErrExtractConflict) {
		t.Fatalf("err = %v, want ErrExtractConflict", err)
	}
	if got := readFile(t, env.target, "c.txt"); got != "already here\n" {
		t.Errorf("target c.txt clobbered: %q", got)
	}
}

func TestExtractResolveWritesMarkersAndKeepsSource(t *testing.T) {
	env := setupExtract(t)
	writeFile(t, env.source, "a.txt", "line1\nSRC\nline3\n")
	writeFile(t, env.target, "a.txt", "line1\nTGT\nline3\n")

	result, err := Extract(t.Context(), domain.ExtractParams{
		SourcePath:   env.source,
		SourceBranch: "main",
		TargetPath:   env.target,
		TargetBranch: "feat",
		ConflictMode: domain.OnConflictResolve,
		Files:        []domain.ExtractFile{{Path: "a.txt", Status: domain.ExtractStatusModified}},
	})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	if len(result.Conflicts) != 1 || result.Conflicts[0] != "a.txt" {
		t.Errorf("result.Conflicts = %v, want [a.txt]", result.Conflicts)
	}
	if got := readFile(t, env.target, "a.txt"); !strings.Contains(got, "<<<<<<<") || !strings.Contains(got, ">>>>>>>") {
		t.Errorf("target a.txt missing conflict markers:\n%s", got)
	}
	if got := readFile(t, env.source, "a.txt"); got != "line1\nSRC\nline3\n" {
		t.Errorf("source a.txt changed in resolve mode: %q", got)
	}
}

func TestExtractResolveAppliesCleanFilesAlongsideConflicts(t *testing.T) {
	env := setupExtract(t)
	// a.txt conflicts; b.txt is a clean change (target untouched on b.txt)
	writeFile(t, env.source, "a.txt", "line1\nSRC\nline3\n")
	writeFile(t, env.target, "a.txt", "line1\nTGT\nline3\n")
	writeFile(t, env.source, "b.txt", "base\nappended\n")

	result, err := Extract(t.Context(), domain.ExtractParams{
		SourcePath:   env.source,
		SourceBranch: "main",
		TargetPath:   env.target,
		TargetBranch: "feat",
		ConflictMode: domain.OnConflictResolve,
		Files: []domain.ExtractFile{
			{Path: "a.txt", Status: domain.ExtractStatusModified},
			{Path: "b.txt", Status: domain.ExtractStatusModified},
		},
	})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	if len(result.Conflicts) != 1 || result.Conflicts[0] != "a.txt" {
		t.Errorf("result.Conflicts = %v, want [a.txt]", result.Conflicts)
	}
	if got := readFile(t, env.target, "b.txt"); got != "base\nappended\n" {
		t.Errorf("clean file b.txt not applied to target: %q", got)
	}
	// source fully intact (nothing removed in resolve mode)
	if got := readFile(t, env.source, "b.txt"); got != "base\nappended\n" {
		t.Errorf("source b.txt should be kept: %q", got)
	}
}

func TestExtractGuards(t *testing.T) {
	env := setupExtract(t)

	_, err := Extract(t.Context(), domain.ExtractParams{SourcePath: env.source, TargetPath: env.source})
	if !errors.Is(err, domain.ErrSameWorktree) {
		t.Errorf("same worktree: err = %v", err)
	}

	_, err = Extract(t.Context(), domain.ExtractParams{SourcePath: env.source, TargetPath: env.target})
	if !errors.Is(err, domain.ErrNoFilesSelected) {
		t.Errorf("no files: err = %v", err)
	}
}

func TestExtractResolveMergesUntrackedCollision(t *testing.T) {
	env := setupExtract(t)
	writeFile(t, env.source, "c.txt", "from source\n")
	writeFile(t, env.target, "c.txt", "already here\n")

	result, err := Extract(t.Context(), domain.ExtractParams{
		SourcePath:   env.source,
		SourceBranch: "main",
		TargetPath:   env.target,
		TargetBranch: "feat",
		ConflictMode: domain.OnConflictResolve,
		Files:        []domain.ExtractFile{{Path: "c.txt", Status: domain.ExtractStatusUntracked}},
	})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	if len(result.Conflicts) != 1 || result.Conflicts[0] != "c.txt" {
		t.Errorf("result.Conflicts = %v, want [c.txt]", result.Conflicts)
	}
	got := readFile(t, env.target, "c.txt")
	if !strings.Contains(got, "<<<<<<<") || !strings.Contains(got, ">>>>>>>") {
		t.Errorf("target c.txt missing conflict markers:\n%s", got)
	}
	if !strings.Contains(got, "from source") || !strings.Contains(got, "already here") {
		t.Errorf("target c.txt lost a side of the merge:\n%s", got)
	}
	if src := readFile(t, env.source, "c.txt"); src != "from source\n" {
		t.Errorf("source c.txt changed in resolve mode: %q", src)
	}
}

func TestExtractUntrackedIdenticalContentIsNotAConflict(t *testing.T) {
	env := setupExtract(t)
	writeFile(t, env.source, "c.txt", "same bytes\n")
	writeFile(t, env.target, "c.txt", "same bytes\n")

	result, err := Extract(t.Context(), domain.ExtractParams{
		SourcePath: env.source,
		TargetPath: env.target,
		Files:      []domain.ExtractFile{{Path: "c.txt", Status: domain.ExtractStatusUntracked}},
	})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	if len(result.Conflicts) != 0 {
		t.Errorf("result.Conflicts = %v, want none for identical content", result.Conflicts)
	}
	if got := readFile(t, env.target, "c.txt"); got != "same bytes\n" {
		t.Errorf("target c.txt = %q", got)
	}
	if fileExists(env.source, "c.txt") {
		t.Error("source c.txt still present after a move")
	}
}

func TestExtractUntrackedBinaryCollisionAbortsEvenInResolveMode(t *testing.T) {
	env := setupExtract(t)
	writeFile(t, env.source, "blob.bin", "sourc\x00e bytes")
	writeFile(t, env.target, "blob.bin", "targe\x00t bytes")

	_, err := Extract(t.Context(), domain.ExtractParams{
		SourcePath:   env.source,
		TargetPath:   env.target,
		TargetBranch: "feat",
		ConflictMode: domain.OnConflictResolve,
		Files:        []domain.ExtractFile{{Path: "blob.bin", Status: domain.ExtractStatusUntracked}},
	})
	if !errors.Is(err, domain.ErrExtractConflict) {
		t.Fatalf("err = %v, want ErrExtractConflict", err)
	}
	if got := readFile(t, env.target, "blob.bin"); got != "targe\x00t bytes" {
		t.Errorf("target blob.bin clobbered with markers: %q", got)
	}
}

func TestExtractMovesSingleFileOutOfNewDirectory(t *testing.T) {
	env := setupExtract(t)
	writeFile(t, env.source, "newmod/x.go", "package newmod\n")
	writeFile(t, env.source, "newmod/sub/y.go", "package sub\n")

	_, err := Extract(t.Context(), domain.ExtractParams{
		SourcePath: env.source,
		TargetPath: env.target,
		Files:      []domain.ExtractFile{{Path: "newmod/sub/y.go", Status: domain.ExtractStatusUntracked}},
	})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	if got := readFile(t, env.target, "newmod/sub/y.go"); got != "package sub\n" {
		t.Errorf("target newmod/sub/y.go = %q", got)
	}
	if fileExists(env.source, "newmod/sub/y.go") {
		t.Error("source still has the moved file")
	}
	if !fileExists(env.source, "newmod/x.go") {
		t.Error("sibling newmod/x.go must stay in the source")
	}
	if fileExists(env.source, "newmod/sub") {
		t.Error("emptied newmod/sub must be pruned from the source")
	}
	if !fileExists(env.source, "newmod") {
		t.Error("newmod still holds x.go and must not be pruned")
	}
}

func TestExtractMovesUntrackedPathWithSpaces(t *testing.T) {
	env := setupExtract(t)
	writeFile(t, env.source, "a b.txt", "spaced\n")

	_, err := Extract(t.Context(), domain.ExtractParams{
		SourcePath: env.source,
		TargetPath: env.target,
		Files:      []domain.ExtractFile{{Path: "a b.txt", Status: domain.ExtractStatusUntracked}},
	})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	if got := readFile(t, env.target, "a b.txt"); got != "spaced\n" {
		t.Errorf("target %q = %q", "a b.txt", got)
	}
	if fileExists(env.source, "a b.txt") {
		t.Error("source still has the moved file")
	}
}

func TestExtractMovesRenamedFile(t *testing.T) {
	env := setupExtract(t)
	gitRun(t, env.source, "mv", "a.txt", "renamed.txt")

	_, err := Extract(t.Context(), domain.ExtractParams{
		SourcePath: env.source,
		TargetPath: env.target,
		Files: []domain.ExtractFile{
			{Path: "renamed.txt", OrigPath: "a.txt", Status: domain.ExtractStatusRenamed},
		},
	})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	if got := readFile(t, env.target, "renamed.txt"); got != "line1\nline2\nline3\n" {
		t.Errorf("target renamed.txt = %q", got)
	}
	if fileExists(env.target, "a.txt") {
		t.Error("target still has the pre-rename path: the deletion half of the patch was lost")
	}
	if status := gitStatus(t, env.source); status != "" {
		t.Errorf("source not restored to HEAD after moving the rename: %q", status)
	}
}

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, dir, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

func fileExists(dir, rel string) bool {
	_, err := os.Stat(filepath.Join(dir, rel))
	return err == nil
}

func gitStatus(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git status: %v", err)
	}
	return string(out)
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", full...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %s: %v", args, out, err)
	}
}
