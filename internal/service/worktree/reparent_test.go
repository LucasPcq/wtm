package worktree

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func seedMeta(t *testing.T, stateDir, branch string, meta domain.WorktreeMetadata) {
	t.Helper()
	if err := writeMetadata(rules.WorktreeMetaDir(stateDir, branch), meta); err != nil {
		t.Fatalf("seed metadata for %s: %v", branch, err)
	}
}

func TestSetSourceBranchPreservesOtherFields(t *testing.T) {
	stateDir := t.TempDir()
	seedMeta(t, stateDir, "dev/b", domain.WorktreeMetadata{
		SourceBranch: "dev/a",
		CreatedAt:    "2026-04-01T18:00:00Z",
		EnvStrategy:  domain.EnvStrategyParent,
	})

	res, err := setSourceBranch(setSourceBranchParams{StateDir: stateDir, Branch: "dev/b", NewParent: "feat"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.OldParent != "dev/a" || res.NewParent != "feat" {
		t.Fatalf("unexpected result: %+v", res)
	}

	meta, err := loadMetadata(stateDir, "dev/b")
	if err != nil {
		t.Fatalf("reload metadata: %v", err)
	}
	if meta.SourceBranch != "feat" {
		t.Errorf("source_branch = %q, want feat", meta.SourceBranch)
	}
	if meta.CreatedAt != "2026-04-01T18:00:00Z" {
		t.Errorf("CreatedAt not preserved: %q", meta.CreatedAt)
	}
	if meta.EnvStrategy != domain.EnvStrategyParent {
		t.Errorf("EnvStrategy not preserved: %q", meta.EnvStrategy)
	}
}

func TestApplyReparentsMovesEachChild(t *testing.T) {
	stateDir := t.TempDir()
	seedMeta(t, stateDir, "dev/b", domain.WorktreeMetadata{SourceBranch: "dev/a", CreatedAt: "x"})
	seedMeta(t, stateDir, "dev/c", domain.WorktreeMetadata{SourceBranch: "dev/a", CreatedAt: "x"})

	moves := []domain.ReparentResult{
		{Branch: "dev/b", OldParent: "dev/a", NewParent: "feat"},
		{Branch: "dev/c", OldParent: "dev/a", NewParent: "feat"},
	}

	applied, err := ApplyReparents(ApplyReparentsParams{Reparents: moves, StateDir: stateDir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(applied) != 2 {
		t.Fatalf("expected 2 reparented children, got %d", len(applied))
	}

	for _, branch := range []string{"dev/b", "dev/c"} {
		meta, err := loadMetadata(stateDir, branch)
		if err != nil {
			t.Fatalf("reload %s: %v", branch, err)
		}
		if meta.SourceBranch != "feat" {
			t.Errorf("%s source_branch = %q, want feat", branch, meta.SourceBranch)
		}
	}
}
