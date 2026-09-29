package worktree

import (
	"os"
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

func relocateTo(t *testing.T, repo ordinalRepo, basePath string) domain.RelocateResult {
	t.Helper()
	result, err := Relocate(RelocateParams{
		ProjectDir:     repo.dir,
		StateDir:       repo.stateDir,
		Config:         domain.Config{Project: domain.ProjectConfig{Worktrees: domain.WorktreesConfig{BasePath: basePath, BaseBranch: "main"}}},
		TargetBasePath: basePath,
		BaseBranch:     "main",
	})
	if err != nil {
		t.Fatalf("Relocate: %v", err)
	}
	return result
}

// Adopting a worktree that already ran jobs completes its record: what it
// learned about its ports, its isolation and the namespaces it holds is what
// clean relies on, and a rewrite from scratch forgot all three.
func TestRelocateAdoptionKeepsWhatMetaJSONAlreadyHolds(t *testing.T) {
	globaldir.Isolate(t)
	repo := newOrdinalRepo(t)
	repo.addWorktree(t, "feat/x")
	held := domain.WorktreeMetadata{Ordinal: 3, Isolation: domain.IsolationVerbatim, Namespaces: []string{"postgres"}}
	if err := writeMetadata(filepath.Join(repo.stateDir, domain.WorktreesSubdir, "feat%2Fx"), held); err != nil {
		t.Fatal(err)
	}

	result := relocateTo(t, repo, "../trees")
	if len(result.Steps) != 1 || result.Steps[0].Status != domain.RelocateStatusMovedAdopted {
		t.Fatalf("steps = %+v, want feat/x moved and adopted", result.Steps)
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
func TestRelocateRefusesToMoveAWorktreeWithJobs(t *testing.T) {
	globaldir.Isolate(t)
	repo := newOrdinalRepo(t)
	repo.addWorktree(t, "feat/x")
	path := gitPathOf(t, repo, "feat/x")
	if err := process.NewStateStore(process.StatePath()).Save([]domain.JobRecord{{Name: "api", WorkDir: path}}); err != nil {
		t.Fatal(err)
	}

	result := relocateTo(t, repo, "../trees")
	if len(result.Steps) != 1 || result.Steps[0].Status != domain.RelocateStatusBlockedJobs {
		t.Fatalf("steps = %+v, want feat/x blocked by its jobs", result.Steps)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("the worktree moved anyway: %v", err)
	}
}
