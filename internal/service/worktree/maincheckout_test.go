package worktree_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

// A shared job runs in the main checkout, so resolving it has to work from any
// worktree — including one that is not it.
func TestMainCheckoutFoundFromALinkedWorktree(t *testing.T) {
	repo := gittest.InitRepo(t)
	linked := filepath.Join(t.TempDir(), "feat-x")
	gittest.Git(t, repo, "worktree", "add", "-b", "feat-x", linked)

	got, err := worktree.MainCheckout(worktree.MainCheckoutParams{ProjectDir: linked})
	if err != nil {
		t.Fatalf("MainCheckout: %v", err)
	}

	want, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	if got != want {
		t.Errorf("MainCheckout = %q, want %q", got, want)
	}
}

func TestMainCheckoutFoundFromTheMainCheckoutItself(t *testing.T) {
	repo := gittest.InitRepo(t)
	got, err := worktree.MainCheckout(worktree.MainCheckoutParams{ProjectDir: repo})
	if err != nil {
		t.Fatalf("MainCheckout: %v", err)
	}
	want, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	if got != want {
		t.Errorf("MainCheckout = %q, want %q", got, want)
	}
}

func TestMainCheckoutErrorsOutsideARepository(t *testing.T) {
	_, err := worktree.MainCheckout(worktree.MainCheckoutParams{ProjectDir: t.TempDir()})
	if err == nil {
		t.Fatal("MainCheckout outside a repository = nil error")
	}
	if !errors.Is(err, domain.ErrNoMainCheckout) && err == nil {
		t.Errorf("err = %v", err)
	}
}
