package worktree

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

// branchExists reports whether a local branch is still present in the repo.
func branchExists(t *testing.T, dir, branch string) bool {
	t.Helper()
	cmd := exec.Command("git", "branch", "--list", branch)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git branch --list %s: %s: %v", branch, out, err)
	}
	return strings.TrimSpace(string(out)) != ""
}

func TestApplyReparentsMovesEachToItsParent(t *testing.T) {
	stateDir := t.TempDir()
	seedMeta(t, stateDir, "dev/b", domain.WorktreeMetadata{SourceBranch: "dev/a", CreatedAt: "x"})
	seedMeta(t, stateDir, "dev/c", domain.WorktreeMetadata{SourceBranch: "dev/x", CreatedAt: "x"})

	applied, err := ApplyReparents(ApplyReparentsParams{
		StateDir: stateDir,
		Reparents: []domain.ReparentResult{
			{Branch: "dev/b", OldParent: "dev/a", NewParent: "main"},
			{Branch: "dev/c", OldParent: "dev/x", NewParent: "feat"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(applied) != 2 {
		t.Fatalf("expected 2 applied, got %d", len(applied))
	}

	b, _ := loadMetadata(stateDir, "dev/b")
	if b.SourceBranch != "main" {
		t.Errorf("dev/b parent = %q, want main", b.SourceBranch)
	}
	c, _ := loadMetadata(stateDir, "dev/c")
	if c.SourceBranch != "feat" {
		t.Errorf("dev/c parent = %q, want feat", c.SourceBranch)
	}
}

// TestPlanPruneGHBased drives the full classification path (List → ClassifyPrune)
// with injected PR state: a worktree whose PR is merged is flagged under --merged,
// while a freshly-created worktree with no PR is never flagged — the LUC-111 fix
// (detection is GitHub-truth, not local commit topology).
func TestPlanPruneGHBased(t *testing.T) {
	source := gittest.InitRepo(t)
	stateDir := t.TempDir()

	mergedPath := filepath.Join(t.TempDir(), "feat-merged")
	freshPath := filepath.Join(t.TempDir(), "feat-fresh")
	gitRun(t, source, "worktree", "add", "-q", "-b", "feat-merged", mergedPath, "HEAD")
	gitRun(t, source, "worktree", "add", "-q", "-b", "feat-fresh", freshPath, "HEAD")

	params := domain.PruneParams{
		ProjectDir: source,
		StateDir:   stateDir,
		Config:     domain.Config{Project: domain.ProjectConfig{Worktrees: domain.WorktreesConfig{BaseBranch: "main"}}},
		BaseBranch: "main",
		Merged:     true,
	}
	prs := []domain.PRInfo{{Branch: "feat-merged", State: domain.PRStateMerged}}

	plan, err := PlanPrune(PlanPruneParams{Prune: params, PRs: func() []domain.PRInfo { return prs }})
	if err != nil {
		t.Fatalf("PlanPrune: %v", err)
	}

	sel := map[string]string{}
	for _, c := range plan.Selected {
		sel[c.Branch] = c.Reason
	}
	if sel["feat-merged"] != domain.PruneReasonPRMerged {
		t.Errorf("feat-merged should be flagged PR merged, selected=%v", sel)
	}
	if _, ok := sel["feat-fresh"]; ok {
		t.Errorf("feat-fresh (no PR) must never be flagged merged")
	}
}
