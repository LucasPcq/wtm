package infra

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

func TestRecentCommits(t *testing.T) {
	dir := gittest.InitRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("un"), 0o644); err != nil {
		t.Fatal(err)
	}
	gittest.Git(t, dir, "add", ".")
	gittest.Git(t, dir, "commit", "-m", "feat: premier ajout")

	commits, err := RecentCommits(t.Context(), RecentCommitsParams{WorktreePath: dir, Limit: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(commits) == 0 {
		t.Fatal("aucun commit retourné")
	}
	if commits[0].Subject != "feat: premier ajout" {
		t.Errorf("Subject = %q, want %q", commits[0].Subject, "feat: premier ajout")
	}
	if len(commits[0].SHA) != 7 {
		t.Errorf("SHA = %q, want 7 caractères", commits[0].SHA)
	}
	if commits[0].At.IsZero() {
		t.Error("At ne doit pas être zéro")
	}
	if commits[0].Author == "" {
		t.Error("Author ne doit pas être vide")
	}
}

func TestRecentCommitsSubjectWithSeparator(t *testing.T) {
	dir := gittest.InitRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("deux"), 0o644); err != nil {
		t.Fatal(err)
	}
	gittest.Git(t, dir, "add", ".")
	gittest.Git(t, dir, "commit", "-m", "fix: garder a|b intact")

	commits, err := RecentCommits(t.Context(), RecentCommitsParams{WorktreePath: dir, Limit: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if commits[0].Subject != "fix: garder a|b intact" {
		t.Errorf("Subject = %q — le séparateur de champ ne doit pas couper le sujet", commits[0].Subject)
	}
}

func TestDiffShortstat(t *testing.T) {
	dir := gittest.InitRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "c.txt"), []byte("l1\nl2\nl3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gittest.Git(t, dir, "add", ".")
	gittest.Git(t, dir, "commit", "-m", "seed")
	if err := os.WriteFile(filepath.Join(dir, "c.txt"), []byte("l1\nl2\nl3\nl4\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stat, err := DiffShortstat(t.Context(), DiffShortstatParams{WorktreePath: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stat.Insertions != 1 {
		t.Errorf("Insertions = %d, want 1", stat.Insertions)
	}
}

func TestBranchDiffShortstat(t *testing.T) {
	dir := gittest.InitRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "c.txt"), []byte("l1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gittest.Git(t, dir, "add", ".")
	gittest.Git(t, dir, "commit", "-m", "seed")
	gittest.Git(t, dir, "checkout", "-b", "feat/x")
	if err := os.WriteFile(filepath.Join(dir, "c.txt"), []byte("l1\nl2\nl3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gittest.Git(t, dir, "add", ".")
	gittest.Git(t, dir, "commit", "-m", "feat: two more lines")

	stat, err := BranchDiffShortstat(t.Context(), BranchDiffShortstatParams{WorktreePath: dir, Base: "main", Branch: "feat/x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stat.Insertions != 2 {
		t.Errorf("Insertions = %d, want 2", stat.Insertions)
	}
	if stat.Deletions != 0 {
		t.Errorf("Deletions = %d, want 0", stat.Deletions)
	}
	if stat.FilesChanged != 1 {
		t.Errorf("FilesChanged = %d, want 1", stat.FilesChanged)
	}
}

func TestLastFetchAtWithoutFetchHead(t *testing.T) {
	dir := gittest.InitRepo(t)
	if got := LastFetchAt(t.Context(), LastFetchAtParams{ProjectDir: dir}); !got.IsZero() {
		t.Errorf("LastFetchAt = %v, want zéro quand FETCH_HEAD est absent", got)
	}
}

func TestRecentCommitsNoCommits(t *testing.T) {
	dir := t.TempDir()
	gittest.Git(t, dir, "init", "-b", "main")

	if _, err := RecentCommits(t.Context(), RecentCommitsParams{WorktreePath: dir, Limit: 5}); err == nil {
		t.Fatal("attendu une erreur sur un dépôt sans commit")
	}
}
