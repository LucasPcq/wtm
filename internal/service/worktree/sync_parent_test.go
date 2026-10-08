package worktree

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

// parentUpdateFor returns the recorded update for a parent branch, or false.
func parentUpdateFor(result domain.SyncResult, branch string) (domain.ParentUpdate, bool) {
	for _, update := range result.ParentUpdates {
		if update.Branch == branch {
			return update, true
		}
	}
	return domain.ParentUpdate{}, false
}

// setupBranchOnlyParent builds main → feature (a branch with NO worktree) → dev
// (a worktree), then pushes everything and advances origin/feature beyond the
// local feature ref. It returns the repo dir, the state dir and dev's path.
func setupBranchOnlyParent(t *testing.T) (dir, stateDir, devPath string) {
	t.Helper()
	dir = gittest.InitRepo(t)
	stateDir = filepath.Join(dir, ".git", "wtm")
	devPath = filepath.Join(t.TempDir(), "dev")

	initRemote(t, dir)

	// feature exists only as a branch: it is never checked out in a worktree.
	git(t, dir, "branch", "feature", "main")
	git(t, dir, "push", "origin", "feature")

	git(t, dir, "worktree", "add", "-b", "dev", devPath, "feature")
	commitFile(t, devPath, "dev.txt", "dev work")
	git(t, devPath, "push", "-u", "origin", "dev")

	// Someone else advances origin/feature; the local feature ref stays behind.
	advanceRemoteBranch(t, dir, "feature", "feature", "remote-feature.txt")

	writeMeta(t, stateDir, "dev", "feature")
	return dir, stateDir, devPath
}

// TestSyncFastForwardsBranchOnlyParent is the regression test for a parent that
// has no worktree of its own: it can never be a step in the cascade, so nothing
// ever refreshed it and the child was rebased onto a stale ref while sync
// reported "up to date".
func TestSyncFastForwardsBranchOnlyParent(t *testing.T) {
	dir, stateDir, devPath := setupBranchOnlyParent(t)

	result, err := Sync(t.Context(), SyncParams{
		ProjectDir:         dir,
		StateDir:           stateDir,
		BaseBranch:         "main",
		SelectedBranches:   []string{"dev"},
		FastForwardParents: true,
	})
	if err != nil {
		t.Fatalf("Sync error: %v", err)
	}

	if local, remote := git(t, dir, "rev-parse", "feature"), git(t, dir, "rev-parse", "origin/feature"); local != remote {
		t.Fatalf("feature = %s, want origin/feature %s (parent should have been fast-forwarded)", local, remote)
	}
	if _, statErr := os.Stat(filepath.Join(devPath, "remote-feature.txt")); statErr != nil {
		t.Fatal("dev should carry the parent's remote-only commit after rebasing onto the refreshed parent")
	}
	if !isAncestor(t, dir, "feature", "dev") {
		t.Fatal("feature should be an ancestor of dev after sync")
	}

	update, ok := parentUpdateFor(result, "feature")
	if !ok {
		t.Fatalf("no parent update recorded for feature (%+v)", result.ParentUpdates)
	}
	if update.Status != domain.ParentFastForwarded {
		t.Fatalf("feature parent status = %q, want %q", update.Status, domain.ParentFastForwarded)
	}
}

// TestSyncReportsStaleParentWhenNotAsked verifies the safe default: without the
// opt-in, a stale parent is left untouched — but it is reported, so the run no
// longer claims the child is up to date while hiding that its parent is behind.
func TestSyncReportsStaleParentWhenNotAsked(t *testing.T) {
	dir, stateDir, devPath := setupBranchOnlyParent(t)
	featureBefore := git(t, dir, "rev-parse", "feature")

	result, err := Sync(t.Context(), SyncParams{
		ProjectDir:       dir,
		StateDir:         stateDir,
		BaseBranch:       "main",
		SelectedBranches: []string{"dev"},
	})
	if err != nil {
		t.Fatalf("Sync error: %v", err)
	}

	if after := git(t, dir, "rev-parse", "feature"); after != featureBefore {
		t.Fatalf("feature moved to %s without the opt-in (was %s)", after, featureBefore)
	}
	if _, statErr := os.Stat(filepath.Join(devPath, "remote-feature.txt")); statErr == nil {
		t.Fatal("dev must not pick up the parent's remote commits without the opt-in")
	}

	update, ok := parentUpdateFor(result, "feature")
	if !ok {
		t.Fatalf("stale parent feature should be reported (%+v)", result.ParentUpdates)
	}
	if update.Status != domain.ParentBehind {
		t.Fatalf("feature parent status = %q, want %q", update.Status, domain.ParentBehind)
	}
}

// TestSyncFastForwardsUnselectedParentWorktree covers the same gap for a parent
// that does have a worktree but was not part of the selection: it is not a step
// either, so it was never refreshed.
func TestSyncFastForwardsUnselectedParentWorktree(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	trees := t.TempDir()
	featPath := filepath.Join(trees, "feat")
	devPath := filepath.Join(trees, "dev")

	initRemote(t, dir)

	git(t, dir, "worktree", "add", "-b", "feat", featPath, "main")
	commitFile(t, featPath, "feat.txt", "feat work")
	git(t, featPath, "push", "-u", "origin", "feat")

	git(t, dir, "worktree", "add", "-b", "dev", devPath, "feat")
	commitFile(t, devPath, "dev.txt", "dev work")
	git(t, devPath, "push", "-u", "origin", "dev")

	advanceRemoteBranch(t, dir, "feat", "feat", "remote-feat.txt")

	writeMeta(t, stateDir, "feat", "main")
	writeMeta(t, stateDir, "dev", "feat")

	result, err := Sync(t.Context(), SyncParams{
		ProjectDir:         dir,
		StateDir:           stateDir,
		BaseBranch:         "main",
		SelectedBranches:   []string{"dev"},
		FastForwardParents: true,
	})
	if err != nil {
		t.Fatalf("Sync error: %v", err)
	}

	if len(result.Steps) != 1 || result.Steps[0].Branch != "dev" {
		t.Fatalf("expected a single dev step, got %+v", result.Steps)
	}
	if local, remote := git(t, dir, "rev-parse", "feat"), git(t, dir, "rev-parse", "origin/feat"); local != remote {
		t.Fatalf("feat = %s, want origin/feat %s (unselected parent should have been fast-forwarded)", local, remote)
	}
	if _, statErr := os.Stat(filepath.Join(devPath, "remote-feat.txt")); statErr != nil {
		t.Fatal("dev should carry the parent's remote-only commit")
	}

	update, ok := parentUpdateFor(result, "feat")
	if !ok || update.Status != domain.ParentFastForwarded {
		t.Fatalf("feat should be reported fast-forwarded, got %+v", result.ParentUpdates)
	}
}

// TestSyncDivergedParentIsNeverFastForwarded verifies the fast-forward stays
// strictly non-destructive: a parent whose local and remote both moved is left
// exactly as it is, even with the opt-in, and is reported as diverged.
func TestSyncDivergedParentIsNeverFastForwarded(t *testing.T) {
	dir, stateDir, _ := setupBranchOnlyParent(t)

	// The local feature ref also moves, so it can no longer fast-forward. The
	// throwaway worktree is detached, so feature stays free to be moved from here.
	localOnly := filepath.Join(t.TempDir(), "localfeature")
	git(t, dir, "worktree", "add", "--detach", localOnly, "feature")
	commitFile(t, localOnly, "local-feature.txt", "local only")
	localTip := git(t, localOnly, "rev-parse", "HEAD")
	git(t, dir, "worktree", "remove", "--force", localOnly)
	git(t, dir, "branch", "-f", "feature", localTip)
	featureBefore := git(t, dir, "rev-parse", "feature")

	result, err := Sync(t.Context(), SyncParams{
		ProjectDir:         dir,
		StateDir:           stateDir,
		BaseBranch:         "main",
		SelectedBranches:   []string{"dev"},
		FastForwardParents: true,
	})
	if err != nil {
		t.Fatalf("Sync error: %v", err)
	}

	if after := git(t, dir, "rev-parse", "feature"); after != featureBefore {
		t.Fatalf("diverged parent feature moved to %s (was %s) — must be left untouched", after, featureBefore)
	}

	update, ok := parentUpdateFor(result, "feature")
	if !ok || update.Status != domain.ParentDiverged {
		t.Fatalf("feature should be reported diverged, got %+v", result.ParentUpdates)
	}
}

// TestSyncParentAlreadyInCascadeIsNotDoubleHandled verifies a parent that IS a
// step keeps being refreshed by its own step (integrateRemote) and is not also
// reported as a parent update — the two paths must not overlap.
func TestSyncParentAlreadyInCascadeIsNotDoubleHandled(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	trees := t.TempDir()
	featPath := filepath.Join(trees, "feat")
	devPath := filepath.Join(trees, "dev")

	initRemote(t, dir)

	git(t, dir, "worktree", "add", "-b", "feat", featPath, "main")
	commitFile(t, featPath, "feat.txt", "feat work")
	git(t, featPath, "push", "-u", "origin", "feat")

	git(t, dir, "worktree", "add", "-b", "dev", devPath, "feat")
	commitFile(t, devPath, "dev.txt", "dev work")

	advanceRemoteBranch(t, dir, "feat", "feat", "remote-feat.txt")

	writeMeta(t, stateDir, "feat", "main")
	writeMeta(t, stateDir, "dev", "feat")

	result, err := Sync(t.Context(), SyncParams{
		ProjectDir:         dir,
		StateDir:           stateDir,
		BaseBranch:         "main",
		FastForwardParents: true,
	})
	if err != nil {
		t.Fatalf("Sync error: %v", err)
	}

	if _, ok := parentUpdateFor(result, "feat"); ok {
		t.Fatalf("feat is a step of the cascade; it must not also be a parent update (%+v)", result.ParentUpdates)
	}
	if _, statErr := os.Stat(filepath.Join(featPath, "remote-feat.txt")); statErr != nil {
		t.Fatal("feat's own step should have pulled its remote-only commit")
	}
	if !isAncestor(t, dir, "feat", "dev") {
		t.Fatal("feat should be an ancestor of dev after the cascade")
	}
}

// TestSyncDeepCascadeThreeLevels extends the cascade coverage past the existing
// two-level case: main → feat → dev1 → dev2, every level rebased in order onto
// its refreshed parent.
func TestSyncDeepCascadeThreeLevels(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	trees := t.TempDir()

	featPath := filepath.Join(trees, "feat")
	dev1Path := filepath.Join(trees, "dev1")
	dev2Path := filepath.Join(trees, "dev2")

	git(t, dir, "worktree", "add", "-b", "feat", featPath, "main")
	commitFile(t, featPath, "feat.txt", "feat work")
	git(t, dir, "worktree", "add", "-b", "dev1", dev1Path, "feat")
	commitFile(t, dev1Path, "dev1.txt", "dev1 work")
	git(t, dir, "worktree", "add", "-b", "dev2", dev2Path, "dev1")
	commitFile(t, dev2Path, "dev2.txt", "dev2 work")

	commitFile(t, dir, "main.txt", "main moved")

	writeMeta(t, stateDir, "feat", "main")
	writeMeta(t, stateDir, "dev1", "feat")
	writeMeta(t, stateDir, "dev2", "dev1")

	result, err := Sync(t.Context(), SyncParams{ProjectDir: dir, StateDir: stateDir, BaseBranch: "main"})
	if err != nil {
		t.Fatalf("Sync error: %v", err)
	}

	order := make([]string, 0, len(result.Steps))
	for _, step := range result.Steps {
		order = append(order, step.Branch)
		if step.Status != domain.SyncStatusSynced {
			t.Fatalf("%s status = %q, want synced (%+v)", step.Branch, step.Status, result.Steps)
		}
	}
	if len(order) != 3 || order[0] != "feat" || order[1] != "dev1" || order[2] != "dev2" {
		t.Fatalf("cascade order = %v, want [feat dev1 dev2]", order)
	}

	for _, pair := range [][2]string{{"main", "feat"}, {"feat", "dev1"}, {"dev1", "dev2"}, {"main", "dev2"}} {
		if !isAncestor(t, dir, pair[0], pair[1]) {
			t.Fatalf("%s should be an ancestor of %s after the cascade", pair[0], pair[1])
		}
	}
}

// TestSyncLeavesBaseAloneWhenNoStepTargetsIt is the S1 shape: every worktree
// hangs off a branch-only parent, so nothing rebases onto the base. The base
// must then stay out of the run entirely — refreshing it is a side effect the
// user never asked for, and it was the loudest line of the recap.
func TestSyncLeavesBaseAloneWhenNoStepTargetsIt(t *testing.T) {
	dir, stateDir, _ := setupBranchOnlyParent(t)
	advanceRemoteBranch(t, dir, "main", "main", "remote-main.txt")
	mainBefore := git(t, dir, "rev-parse", "main")

	result, err := Sync(t.Context(), SyncParams{
		ProjectDir:       dir,
		StateDir:         stateDir,
		BaseBranch:       "main",
		SelectedBranches: []string{"dev"},
	})
	if err != nil {
		t.Fatalf("Sync error: %v", err)
	}

	if after := git(t, dir, "rev-parse", "main"); after != mainBefore {
		t.Fatalf("main moved to %s (was %s) — no step rebases onto it", after, mainBefore)
	}
	if result.BaseTargeted {
		t.Fatal("BaseTargeted should be false when no step rebases onto the base")
	}
	if result.BaseUpdated {
		t.Fatal("BaseUpdated should be false when the base was never refreshed")
	}
}

// TestSyncRefreshesBaseWhenAStepTargetsIt is the counterpart: as soon as one
// step rebases onto the base, the base is fetched and fast-forwarded as before.
func TestSyncRefreshesBaseWhenAStepTargetsIt(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	featPath := filepath.Join(t.TempDir(), "feat")

	initRemote(t, dir)
	git(t, dir, "worktree", "add", "-b", "feat", featPath, "main")
	commitFile(t, featPath, "feat.txt", "feat work")
	writeMeta(t, stateDir, "feat", "main")

	advanceRemoteBranch(t, dir, "main", "main", "remote-main.txt")
	mainBefore := git(t, dir, "rev-parse", "main")

	result, err := Sync(t.Context(), SyncParams{
		ProjectDir:       dir,
		StateDir:         stateDir,
		BaseBranch:       "main",
		SelectedBranches: []string{"feat"},
	})
	if err != nil {
		t.Fatalf("Sync error: %v", err)
	}

	if after := git(t, dir, "rev-parse", "main"); after == mainBefore {
		t.Fatal("main should have been fast-forwarded — feat rebases onto it")
	}
	if !result.BaseTargeted || !result.BaseUpdated {
		t.Fatalf("base should be targeted and updated, got targeted=%v updated=%v", result.BaseTargeted, result.BaseUpdated)
	}
}

// TestSyncBaseOnlyRefreshKeepsWorking guards the existing feature: a run with no
// rebase steps at all still refreshes the base (that is the whole point of it).
func TestSyncBaseOnlyRefreshKeepsWorking(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	initRemote(t, dir)
	advanceRemoteBranch(t, dir, "main", "main", "remote-main.txt")
	mainBefore := git(t, dir, "rev-parse", "main")

	result, err := Sync(t.Context(), SyncParams{ProjectDir: dir, StateDir: stateDir, BaseBranch: "main"})
	if err != nil {
		t.Fatalf("Sync error: %v", err)
	}

	if after := git(t, dir, "rev-parse", "main"); after == mainBefore {
		t.Fatal("a base-only run should still fast-forward the base")
	}
	if !result.BaseTargeted {
		t.Fatal("BaseTargeted should be true for a base-only refresh")
	}
}

// TestSyncLocallyRebasedParentIsNotReportedDiverged covers a banal workflow: the
// parent was rebased locally, so origin/<parent>'s commits are patch-present but
// neither ref is an ancestor of the other. Calling that "diverged, reconcile it
// manually" on every run teaches the user to ignore the parent block entirely.
func TestSyncLocallyRebasedParentIsNotReportedDiverged(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	devPath := filepath.Join(t.TempDir(), "dev")

	initRemote(t, dir)
	git(t, dir, "branch", "feature", "main")
	git(t, dir, "push", "origin", "feature")

	// feature gets a commit and is published, then rebased locally onto a moved
	// main: same patch, new sha, so local and origin/feature each hold commits the
	// other lacks by sha.
	work := filepath.Join(t.TempDir(), "featwork")
	git(t, dir, "worktree", "add", work, "feature")
	commitFile(t, work, "feature.txt", "feature work")
	git(t, work, "push", "origin", "feature")
	commitFile(t, dir, "main.txt", "main moved")
	git(t, work, "rebase", "main")
	git(t, dir, "worktree", "remove", "--force", work)

	git(t, dir, "worktree", "add", "-b", "dev", devPath, "feature")
	commitFile(t, devPath, "dev.txt", "dev work")
	writeMeta(t, stateDir, "dev", "feature")

	result, err := Sync(t.Context(), SyncParams{
		ProjectDir:       dir,
		StateDir:         stateDir,
		BaseBranch:       "main",
		SelectedBranches: []string{"dev"},
	})
	if err != nil {
		t.Fatalf("Sync error: %v", err)
	}

	if update, ok := parentUpdateFor(result, "feature"); ok {
		t.Fatalf("a locally-rebased parent has nothing to reconcile, got %+v", update)
	}
}

// TestSyncFailedParentFastForwardIsReportedWithReason covers the other half: the
// refresh was asked for and could not happen. Reporting it as plain "behind"
// re-suggests --ff-parents, which was already passed, so the run would be
// replayed identically forever.
func TestSyncFailedParentFastForwardIsReportedWithReason(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	trees := t.TempDir()
	featPath := filepath.Join(trees, "feat")
	devPath := filepath.Join(trees, "dev")

	initRemote(t, dir)
	git(t, dir, "worktree", "add", "-b", "feat", featPath, "main")
	commitFile(t, featPath, "feat.txt", "feat work")
	git(t, featPath, "push", "-u", "origin", "feat")
	git(t, dir, "worktree", "add", "-b", "dev", devPath, "feat")
	commitFile(t, devPath, "dev.txt", "dev work")

	advanceRemoteBranch(t, dir, "feat", "feat", "remote-feat.txt")

	writeMeta(t, stateDir, "feat", "main")
	writeMeta(t, stateDir, "dev", "feat")

	// An untracked file is enough to make the parent worktree dirty.
	if err := os.WriteFile(filepath.Join(featPath, "scratch.txt"), []byte("wip"), 0o644); err != nil {
		t.Fatalf("write scratch: %v", err)
	}

	result, err := Sync(t.Context(), SyncParams{
		ProjectDir:         dir,
		StateDir:           stateDir,
		BaseBranch:         "main",
		SelectedBranches:   []string{"dev"},
		FastForwardParents: true,
	})
	if err != nil {
		t.Fatalf("Sync error: %v", err)
	}

	update, ok := parentUpdateFor(result, "feat")
	if !ok {
		t.Fatalf("the failed fast-forward should be reported (%+v)", result.ParentUpdates)
	}
	if update.Status != domain.ParentFFFailed {
		t.Fatalf("feat status = %q, want %q", update.Status, domain.ParentFFFailed)
	}
	if update.Detail == "" {
		t.Error("a failed fast-forward must carry the reason")
	}
}

func TestClassifyParents(t *testing.T) {
	dir, stateDir, _ := setupBranchOnlyParent(t)

	updates, err := ClassifyParents(t.Context(), ClassifyParentsParams{
		ProjectDir: dir,
		StateDir:   stateDir,
		BaseBranch: "main",
	})
	if err != nil {
		t.Fatalf("ClassifyParents error: %v", err)
	}

	// The inspection is selection-independent: it classifies every recorded parent
	// so a wizard can filter it locally as the selection changes.
	if len(updates) != 1 || updates[0].Branch != "feature" {
		t.Fatalf("updates = %+v, want only feature", updates)
	}
	if updates[0].Status != domain.ParentBehind {
		t.Errorf("feature status = %q, want %q", updates[0].Status, domain.ParentBehind)
	}
	if updates[0].Behind != 1 {
		t.Errorf("feature behind = %d, want 1", updates[0].Behind)
	}

	// It changes nothing.
	if local, remote := git(t, dir, "rev-parse", "feature"), git(t, dir, "rev-parse", "origin/feature"); local == remote {
		t.Error("ClassifyParents must not move the parent")
	}
}

// A dry-run stays offline but must still name the problem it is previewing,
// otherwise the preview hides the very thing the run would report.
func TestSyncDryRunReportsParentsWithoutTouchingAnything(t *testing.T) {
	dir, stateDir, _ := setupBranchOnlyParent(t)
	featureBefore := git(t, dir, "rev-parse", "feature")

	result, err := Sync(t.Context(), SyncParams{
		ProjectDir:         dir,
		StateDir:           stateDir,
		BaseBranch:         "main",
		SelectedBranches:   []string{"dev"},
		DryRun:             true,
		FastForwardParents: true,
	})
	if err != nil {
		t.Fatalf("Sync error: %v", err)
	}

	update, ok := parentUpdateFor(result, "feature")
	if !ok {
		t.Fatalf("a dry-run should still report the stale parent (%+v)", result.ParentUpdates)
	}
	// Even with the opt-in: a dry-run never moves anything.
	if update.Status != domain.ParentBehind {
		t.Errorf("dry-run status = %q, want %q", update.Status, domain.ParentBehind)
	}
	if after := git(t, dir, "rev-parse", "feature"); after != featureBefore {
		t.Fatalf("dry-run moved feature to %s (was %s)", after, featureBefore)
	}
}

// --all covers the whole forest including its root: it keeps refreshing the base
// even in a tree where every worktree hangs off some other parent. Only an
// explicit selection leaves the base out.
func TestSyncAllStillRefreshesTheBase(t *testing.T) {
	dir, stateDir, _ := setupBranchOnlyParent(t)
	advanceRemoteBranch(t, dir, "main", "main", "remote-main.txt")
	mainBefore := git(t, dir, "rev-parse", "main")

	result, err := Sync(t.Context(), SyncParams{ProjectDir: dir, StateDir: stateDir, BaseBranch: "main"})
	if err != nil {
		t.Fatalf("Sync error: %v", err)
	}

	if !result.BaseTargeted {
		t.Fatal("--all should keep the base in scope")
	}
	if after := git(t, dir, "rev-parse", "main"); after == mainBefore {
		t.Fatal("--all should still fast-forward the base")
	}
}
