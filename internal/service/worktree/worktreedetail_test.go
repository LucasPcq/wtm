package worktree

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

func TestDetailReadsCommitsAndChanges(t *testing.T) {
	dir := gittest.InitRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("un"), 0o644); err != nil {
		t.Fatal(err)
	}
	gittest.Git(t, dir, "add", ".")
	gittest.Git(t, dir, "commit", "-m", "feat: seed")
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("deux"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := Detail(t.Context(), DetailParams{
		ProjectDir: dir,
		Status:     domain.WorktreeStatus{Branch: "main", Path: dir, IsDirty: true},
		Children:   []string{"feat/enfant"},
		Commits:    domain.DashboardDetailCommits,
	})

	if len(got.Commits) == 0 {
		t.Fatal("Commits vide")
	}
	if got.Commits[0].Subject != "feat: seed" {
		t.Errorf("Commits[0].Subject = %q, want %q", got.Commits[0].Subject, "feat: seed")
	}
	if got.Changes.Untracked != 1 {
		t.Errorf("Changes.Untracked = %d, want 1", got.Changes.Untracked)
	}
	if len(got.Children) != 1 || got.Children[0] != "feat/enfant" {
		t.Errorf("Children = %v, want [feat/enfant]", got.Children)
	}
	if len(got.Failures) != 0 {
		t.Errorf("Failures = %v, want vide", got.Failures)
	}
}

func TestDetailBlockersFromMemoryNotFromGH(t *testing.T) {
	dir := gittest.InitRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("un"), 0o644); err != nil {
		t.Fatal(err)
	}
	gittest.Git(t, dir, "add", ".")
	gittest.Git(t, dir, "commit", "-m", "seed")

	got := Detail(t.Context(), DetailParams{
		ProjectDir: dir,
		Status:     domain.WorktreeStatus{Branch: "feat/x", Path: dir, IsDirty: true, IsLocked: true},
		PRs:        []domain.PRInfo{{Branch: "feat/x", Number: 7, URL: "https://example/7"}},
		Commits:    domain.DashboardDetailCommits,
	})

	keys := map[string]bool{}
	for _, blocker := range got.Blockers {
		keys[blocker.Key] = true
	}
	if !keys[domain.CleanBlockerDirty] {
		t.Error("le worktree sale doit produire un blocker dirty")
	}
	if !keys[domain.CleanBlockerOpenPR] {
		t.Error("la PR passée en entrée doit produire un blocker open-PR, sans appeler gh")
	}
	if !keys[domain.CleanBlockerLocked] {
		t.Error("le worktree verrouillé doit produire un blocker locked")
	}
}

func TestDetailRecordsFailurePerFamily(t *testing.T) {
	got := Detail(t.Context(), DetailParams{
		ProjectDir: t.TempDir(),
		Status:     domain.WorktreeStatus{Branch: "orpheline", Path: filepath.Join(t.TempDir(), "absent")},
		Commits:    domain.DashboardDetailCommits,
	})

	if _, failed := got.Failures[domain.DetailFamilyCommits]; !failed {
		t.Error("un chemin inexistant doit inscrire une panne pour la famille commits")
	}
	if got.Branch != "orpheline" {
		t.Errorf("Branch = %q — le détail reste exploitable malgré la panne", got.Branch)
	}
}

// TestDetailEnvDriftUsesParentPathNotMain pins that DetailParams.ParentPath reaches
// ComputeEnvParams.ParentWorktreePath: under the "parent" env strategy, the parent
// worktree's file (present but missing KEY_A) must be read strictly on its own,
// never silently falling back to main just because ParentPath was left unthreaded.
func TestDetailEnvDriftUsesParentPathNotMain(t *testing.T) {
	mainDir := gittest.InitRepo(t)
	if err := os.WriteFile(filepath.Join(mainDir, ".env"), []byte("KEY_A=main_value\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	childDir := gittest.InitRepo(t)
	if err := os.WriteFile(filepath.Join(childDir, ".env.example"), []byte("KEY_A=placeholder\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	parentDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(parentDir, ".env"), []byte("KEY_B=parent_other\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stateDir := t.TempDir()
	if err := writeMetadata(rules.WorktreeMetaDir(stateDir, "feat/x"), domain.WorktreeMetadata{
		EnvStrategy: domain.EnvStrategyParent,
	}); err != nil {
		t.Fatal(err)
	}

	cfg := domain.Config{Project: domain.ProjectConfig{Env: domain.EnvConfig{
		Files: []domain.EnvFile{{Target: ".env", Template: ".env.example"}},
	}}}

	got := Detail(t.Context(), DetailParams{
		ProjectDir: mainDir,
		StateDir:   stateDir,
		Config:     cfg,
		Status:     domain.WorktreeStatus{Branch: "feat/x", Path: childDir},
		Parent:     "main",
		ParentPath: parentDir,
		Commits:    domain.DashboardDetailCommits,
	})

	if got.EnvDrift.Missing != 1 {
		t.Errorf("EnvDrift.Missing = %d, want 1: KEY_A must stay unresolved (parent worktree has no KEY_A) instead of silently resolving from main, which would happen if ParentPath were not threaded through", got.EnvDrift.Missing)
	}
}

// TestDetailReadsBranchDiffAgainstBase pins that ACTIVITY's title-row diff
// volume is the branch's committed work against Config's base branch, the
// merge-base (three-dot) comparison rather than the working tree's
// uncommitted diff CHANGES already reads.
func TestDetailReadsBranchDiffAgainstBase(t *testing.T) {
	dir := gittest.InitRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("un"), 0o644); err != nil {
		t.Fatal(err)
	}
	gittest.Git(t, dir, "add", ".")
	gittest.Git(t, dir, "commit", "-m", "seed")
	gittest.Git(t, dir, "checkout", "-b", "feat/x")
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("deux\ntrois\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gittest.Git(t, dir, "add", ".")
	gittest.Git(t, dir, "commit", "-m", "feat: ajout")

	cfg := domain.Config{Project: domain.ProjectConfig{Worktrees: domain.WorktreesConfig{BaseBranch: "main"}}}

	got := Detail(t.Context(), DetailParams{
		ProjectDir: dir,
		Config:     cfg,
		Status:     domain.WorktreeStatus{Branch: "feat/x", Path: dir},
		Commits:    domain.DashboardDetailCommits,
	})

	if got.BranchDiff.Insertions != 2 {
		t.Errorf("BranchDiff.Insertions = %d, want 2", got.BranchDiff.Insertions)
	}
	if got.BranchDiff.FilesChanged != 1 {
		t.Errorf("BranchDiff.FilesChanged = %d, want 1", got.BranchDiff.FilesChanged)
	}
	if _, failed := got.Failures[domain.DetailFamilyBranchDiff]; failed {
		t.Errorf("Failures = %v, want no branch_diff failure", got.Failures)
	}
}

// TestDetailSkipsBranchDiffForParent pins that the parent worktree never
// triggers the base-diff read at all — it has no base to diff against — by
// using a path that would otherwise fail the call and asserting no failure
// is recorded for it.
func TestDetailSkipsBranchDiffForParent(t *testing.T) {
	got := Detail(t.Context(), DetailParams{
		ProjectDir: t.TempDir(),
		Status:     domain.WorktreeStatus{Branch: "main", Path: filepath.Join(t.TempDir(), "absent"), IsParent: true},
		Commits:    domain.DashboardDetailCommits,
	})

	if _, failed := got.Failures[domain.DetailFamilyBranchDiff]; failed {
		t.Error("the parent worktree must skip the base-diff read entirely, not fail it")
	}
	if got.BranchDiff != (domain.DiffStat{}) {
		t.Errorf("BranchDiff = %+v, want zero for the parent worktree", got.BranchDiff)
	}
}
