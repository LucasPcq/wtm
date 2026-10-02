package worktree

import (
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/testutil/globaldir"
)

func gitPathOf(t *testing.T, repo ordinalRepo, branch string) string {
	t.Helper()
	worktrees, err := ListAll(ListAllParams{ProjectDir: repo.dir})
	if err != nil {
		t.Fatal(err)
	}
	for _, wt := range worktrees {
		if wt.Branch == branch {
			return wt.Path
		}
	}
	t.Fatalf("no worktree on %s", branch)
	return ""
}

func planAt(t *testing.T, repo ordinalRepo, basePath string) domain.RelocatePlan {
	t.Helper()
	plan, err := PlanRelocate(PlanRelocateParams{
		ProjectDir:     repo.dir,
		StateDir:       repo.stateDir,
		TargetBasePath: basePath,
		BaseBranch:     "main",
	})
	if err != nil {
		t.Fatalf("PlanRelocate: %v", err)
	}
	return plan
}

// Adopting a worktree that already ran jobs completes its record: what it
// learned about its ports, its isolation and the namespaces it holds is what
// clean relies on, and a rewrite from scratch forgot all three.
func TestAdoptKeepsWhatMetaJSONAlreadyHolds(t *testing.T) {
	globaldir.Isolate(t)
	repo := newOrdinalRepo(t)
	repo.addWorktree(t, "feat/x")
	held := domain.WorktreeMetadata{Ordinal: 3, Isolation: domain.IsolationVerbatim, Namespaces: []string{"postgres"}}
	if err := writeMetadata(filepath.Join(repo.stateDir, domain.WorktreesSubdir, "feat%2Fx"), held); err != nil {
		t.Fatal(err)
	}

	if err := Adopt(AdoptParams{StateDir: repo.stateDir, Branch: "feat/x", Parent: "main"}); err != nil {
		t.Fatalf("Adopt: %v", err)
	}

	meta := repo.meta(t, "feat/x")
	if meta.CreatedAt == "" || meta.SourceBranch != "main" {
		t.Errorf("adoption did not record itself: %+v", meta)
	}
	if meta.Ordinal != 3 || meta.Isolation != domain.IsolationVerbatim || len(meta.Namespaces) != 1 {
		t.Errorf("meta.json lost what it held: %+v", meta)
	}
}

// Jobs are keyed on their worktree's path: moving a worktree they run in
// leaves them orphaned under a directory that no longer exists.
func TestPlanRelocateRefusesToMoveAWorktreeWithJobs(t *testing.T) {
	globaldir.Isolate(t)
	repo := newOrdinalRepo(t)
	repo.addWorktree(t, "feat/x")
	path := gitPathOf(t, repo, "feat/x")
	if err := process.NewStateStore(process.StatePath()).Save([]domain.JobRecord{{Name: "api", WorkDir: path}}); err != nil {
		t.Fatal(err)
	}

	plan := planAt(t, repo, "../trees")
	if len(plan.Steps) != 1 || plan.Steps[0].Status != domain.RelocateStatusBlockedJobs {
		t.Fatalf("steps = %+v, want feat/x blocked by its jobs", plan.Steps)
	}
}
