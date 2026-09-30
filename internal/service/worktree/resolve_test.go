package worktree

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestResolveExactMatch(t *testing.T) {
	worktrees := []domain.GitWorktree{
		{Path: "/repo", Branch: "main", IsMain: true},
		{Path: "/trees/feature-auth", Branch: "feature/auth"},
		{Path: "/trees/feature-auth-v2", Branch: "feature/auth-v2"},
	}

	result := resolveWorktree(worktrees, "feature/auth")

	if result.Ambiguous {
		t.Error("expected direct match, not ambiguous")
	}
	if result.Path != "/trees/feature-auth" {
		t.Errorf("expected /trees/feature-auth, got %s", result.Path)
	}
}

func TestResolveSubstringUnique(t *testing.T) {
	worktrees := []domain.GitWorktree{
		{Path: "/repo", Branch: "main", IsMain: true},
		{Path: "/trees/feature-login", Branch: "feature/login"},
	}

	result := resolveWorktree(worktrees, "login")

	if result.Ambiguous {
		t.Error("expected direct match for unique substring")
	}
	if result.Path != "/trees/feature-login" {
		t.Errorf("expected /trees/feature-login, got %s", result.Path)
	}
}

func TestResolveSubstringAmbiguous(t *testing.T) {
	worktrees := []domain.GitWorktree{
		{Path: "/repo", Branch: "main", IsMain: true},
		{Path: "/trees/feature-auth", Branch: "feature/auth"},
		{Path: "/trees/feature-auth-v2", Branch: "feature/auth-v2"},
	}

	result := resolveWorktree(worktrees, "auth")

	if !result.Ambiguous {
		t.Error("expected ambiguous result")
	}
	if len(result.Matches) != 2 {
		t.Errorf("expected 2 matches, got %d", len(result.Matches))
	}
}

func TestResolveEmptyQuery(t *testing.T) {
	worktrees := []domain.GitWorktree{
		{Path: "/repo", Branch: "main", IsMain: true},
		{Path: "/trees/feat", Branch: "feat"},
	}

	result := resolveWorktree(worktrees, "")

	if !result.Ambiguous {
		t.Error("expected ambiguous (all worktrees) for empty query")
	}
	if len(result.Matches) != 2 {
		t.Errorf("expected 2 matches, got %d", len(result.Matches))
	}
}

func TestResolveNotFound(t *testing.T) {
	worktrees := []domain.GitWorktree{
		{Path: "/repo", Branch: "main", IsMain: true},
	}

	result := resolveWorktree(worktrees, "nonexistent")

	if result.Path != "" || result.Ambiguous {
		t.Error("expected empty result for no match")
	}
}

func TestMatchSyncBranchExact(t *testing.T) {
	worktrees := []domain.GitWorktree{
		{Branch: "main", IsMain: true},
		{Branch: "feature/auth"},
		{Branch: "feature/auth-v2"},
	}

	branch, err := matchSyncBranch(worktrees, "feature/auth")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if branch != "feature/auth" {
		t.Fatalf("expected feature/auth, got %s", branch)
	}
}

func TestMatchSyncBranchSubstringUnique(t *testing.T) {
	worktrees := []domain.GitWorktree{
		{Branch: "main", IsMain: true},
		{Branch: "feature/login"},
	}

	branch, err := matchSyncBranch(worktrees, "login")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if branch != "feature/login" {
		t.Fatalf("expected feature/login, got %s", branch)
	}
}

func TestMatchSyncBranchAmbiguous(t *testing.T) {
	worktrees := []domain.GitWorktree{
		{Branch: "feature/auth"},
		{Branch: "feature/auth-v2"},
	}

	if _, err := matchSyncBranch(worktrees, "auth"); err == nil {
		t.Fatal("expected an ambiguity error")
	}
}

func TestMatchSyncBranchNotFound(t *testing.T) {
	worktrees := []domain.GitWorktree{{Branch: "main", IsMain: true}}

	_, err := matchSyncBranch(worktrees, "nope")
	if !errors.Is(err, domain.ErrBranchNotFound) {
		t.Fatalf("expected ErrBranchNotFound, got %v", err)
	}
}

// The positional is a name, and only a name: a surface handing a path — a
// dashboard row passing what it has closest to hand — resolves to nothing and
// refuses the run it was meant to start.
func TestResolveNeverMatchesAPath(t *testing.T) {
	worktrees := []domain.GitWorktree{
		{Path: "/wt/feature-auth", Branch: "feature/auth"},
		{Path: "/wt/feature-login", Branch: "feature/login"},
	}

	if result := resolveWorktree(worktrees, "/wt/feature-auth"); result.Path != "" || result.Ambiguous {
		t.Fatalf("result = %+v, want a path to match nothing", result)
	}
	if result := resolveWorktree(worktrees, "feature/auth"); result.Path != "/wt/feature-auth" {
		t.Fatalf("result = %+v, want the branch to name its worktree", result)
	}
}
