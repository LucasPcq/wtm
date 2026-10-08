package worktree

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s (in %s): %s: %v", strings.Join(args, " "), dir, out, err)
	}
	return strings.TrimSpace(string(out))
}

func commitFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	git(t, dir, "add", name)
	git(t, dir, "commit", "-m", "add "+name)
}

func writeMeta(t *testing.T, stateDir, branch, source string) {
	t.Helper()
	metaDir := rules.WorktreeMetaDir(stateDir, branch)
	if err := os.MkdirAll(metaDir, 0o755); err != nil {
		t.Fatalf("mkdir meta: %v", err)
	}
	data, _ := json.Marshal(domain.WorktreeMetadata{SourceBranch: source})
	if err := os.WriteFile(filepath.Join(metaDir, domain.MetaFileName), data, 0o644); err != nil {
		t.Fatalf("write meta: %v", err)
	}
}

func isAncestor(t *testing.T, dir, ancestor, descendant string) bool {
	t.Helper()
	cmd := exec.Command("git", "merge-base", "--is-ancestor", ancestor, descendant)
	cmd.Dir = dir
	return cmd.Run() == nil
}

// initRemote creates a bare remote, wires it as origin, and pushes main.
func initRemote(t *testing.T, dir string) string {
	t.Helper()
	remote := t.TempDir()
	git(t, remote, "init", "--bare", "-b", "main")
	git(t, dir, "remote", "add", "origin", remote)
	git(t, dir, "push", "origin", "main")
	return remote
}

// advanceRemoteBranch pushes a remote-only commit onto origin/<branch>, starting
// from baseRef, via a throwaway worktree (so origin/<branch> moves ahead of any
// local copy).
func advanceRemoteBranch(t *testing.T, dir, baseRef, branch, file string) {
	t.Helper()
	tmp := filepath.Join(t.TempDir(), "remoteadv")
	wtBranch := "remoteadv-" + branch
	git(t, dir, "worktree", "add", "-b", wtBranch, tmp, baseRef)
	commitFile(t, tmp, file, "remote-only content")
	git(t, tmp, "push", "origin", "HEAD:"+branch)
	git(t, dir, "worktree", "remove", "--force", tmp)
	git(t, dir, "branch", "-D", wtBranch)
}

// TestSyncFastForwardsBranchFromRemote verifies a branch whose origin moved
// ahead (e.g. a PR merged into it) is fast-forwarded before being rebased.
func TestSyncFastForwardsBranchFromRemote(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	trees := t.TempDir()
	featPath := filepath.Join(trees, "feat")

	initRemote(t, dir)

	git(t, dir, "worktree", "add", "-b", "feat", featPath, "main")
	commitFile(t, featPath, "feat.txt", "feat work")
	git(t, featPath, "push", "-u", "origin", "feat")

	// origin/feat advances beyond the local feat tip.
	advanceRemoteBranch(t, dir, "feat", "feat", "remote.txt")
	// main diverges too, so feat must also rebase.
	commitFile(t, dir, "main.txt", "main moved")

	writeMeta(t, stateDir, "feat", "main")

	result, err := Sync(t.Context(), SyncParams{ProjectDir: dir, StateDir: stateDir, BaseBranch: "main"})
	if err != nil {
		t.Fatalf("Sync error: %v", err)
	}

	if result.Steps[0].Status != domain.SyncStatusSynced {
		t.Fatalf("feat status = %q, want synced (%+v)", result.Steps[0].Status, result.Steps)
	}
	if _, statErr := os.Stat(filepath.Join(featPath, "remote.txt")); statErr != nil {
		t.Fatal("remote-only commit should have been pulled into feat before rebasing")
	}
	if !isAncestor(t, dir, "main", "feat") {
		t.Fatal("main should be an ancestor of feat after sync")
	}
}

// TestSyncDivergedBranchIsSkipped verifies a branch whose local and remote both
// moved (no fast-forward) is reported diverged and left untouched.
func TestSyncDivergedBranchIsSkipped(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	trees := t.TempDir()
	featPath := filepath.Join(trees, "feat")

	initRemote(t, dir)

	git(t, dir, "worktree", "add", "-b", "feat", featPath, "main")
	commitFile(t, featPath, "feat.txt", "feat work")
	git(t, featPath, "push", "-u", "origin", "feat")
	featBase := git(t, featPath, "rev-parse", "HEAD")

	// origin/feat gains a remote-only commit…
	advanceRemoteBranch(t, dir, featBase, "feat", "remote.txt")
	// …while local feat gains a different commit (true divergence).
	commitFile(t, featPath, "local.txt", "local work")

	commitFile(t, dir, "main.txt", "main moved")
	writeMeta(t, stateDir, "feat", "main")

	result, err := Sync(t.Context(), SyncParams{ProjectDir: dir, StateDir: stateDir, BaseBranch: "main"})
	if err != nil {
		t.Fatalf("Sync error: %v", err)
	}
	if result.Steps[0].Status != domain.SyncStatusDiverged {
		t.Fatalf("feat status = %q, want diverged (%+v)", result.Steps[0].Status, result.Steps)
	}
}

// TestSyncResyncAfterRebaseIsUpToDateAndPushable verifies that re-running sync
// after a local-only rebase (no push) does NOT report the rebased branch as
// diverged: its remote commits are already integrated (by patch), so it comes
// back up_to_date and still pushable (ahead of origin/<branch>).
func TestSyncResyncAfterRebaseIsUpToDateAndPushable(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	trees := t.TempDir()
	featPath := filepath.Join(trees, "feat")
	dev1Path := filepath.Join(trees, "dev1")

	initRemote(t, dir)

	git(t, dir, "worktree", "add", "-b", "feat", featPath, "main")
	commitFile(t, featPath, "feat.txt", "feat work")
	git(t, featPath, "push", "-u", "origin", "feat")

	git(t, dir, "worktree", "add", "-b", "dev1", dev1Path, "feat")
	commitFile(t, dev1Path, "dev1.txt", "dev1 work")

	// origin/feat advances (e.g. a PR merged into it) and main diverges.
	advanceRemoteBranch(t, dir, "feat", "feat", "remote.txt")
	commitFile(t, dir, "main.txt", "main moved")

	writeMeta(t, stateDir, "feat", "main")
	writeMeta(t, stateDir, "dev1", "feat")

	// First sync: rebases everything locally, no push.
	first, err := Sync(t.Context(), SyncParams{ProjectDir: dir, StateDir: stateDir, BaseBranch: "main"})
	if err != nil {
		t.Fatalf("first Sync error: %v", err)
	}
	if first.Steps[0].Status != domain.SyncStatusSynced {
		t.Fatalf("feat first sync = %q, want synced (%+v)", first.Steps[0].Status, first.Steps)
	}

	// Second sync: nothing new to rebase, and the branch must NOT look diverged.
	second, err := Sync(t.Context(), SyncParams{ProjectDir: dir, StateDir: stateDir, BaseBranch: "main"})
	if err != nil {
		t.Fatalf("second Sync error: %v", err)
	}
	byBranch := map[string]domain.SyncStepResult{}
	for _, step := range second.Steps {
		byBranch[step.Branch] = step
	}
	if got := byBranch["feat"].Status; got != domain.SyncStatusUpToDate {
		t.Fatalf("feat re-sync = %q, want up_to_date (%+v)", got, second.Steps)
	}
	if !byBranch["feat"].PushPending {
		t.Fatal("feat re-sync should be push_pending (local ahead of origin/feat, never pushed)")
	}
	if got := byBranch["dev1"].Status; got != domain.SyncStatusUpToDate {
		t.Fatalf("dev1 re-sync = %q, want up_to_date (%+v)", got, second.Steps)
	}
	if !byBranch["dev1"].PushPending {
		t.Fatal("dev1 re-sync should be push_pending (origin/dev1 does not exist)")
	}
}

// TestSyncCascadeCleanRebase builds main → feat → dev1, advances main, then
// verifies the cascade rebases both branches so main becomes their ancestor.
func TestSyncCascadeCleanRebase(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	trees := t.TempDir()

	featPath := filepath.Join(trees, "feat")
	dev1Path := filepath.Join(trees, "dev1")

	// feat branched from main, with its own commit.
	git(t, dir, "worktree", "add", "-b", "feat", featPath, "main")
	commitFile(t, featPath, "feat.txt", "feat work")

	// dev1 branched from feat, with its own commit.
	git(t, dir, "worktree", "add", "-b", "dev1", dev1Path, "feat")
	commitFile(t, dev1Path, "dev1.txt", "dev1 work")

	// main diverges after the branches were created.
	commitFile(t, dir, "main.txt", "main moved")

	writeMeta(t, stateDir, "feat", "main")
	writeMeta(t, stateDir, "dev1", "feat")

	result, err := Sync(t.Context(), SyncParams{
		ProjectDir: dir,
		StateDir:   stateDir,
		BaseBranch: "main",
	})
	if err != nil {
		t.Fatalf("Sync error: %v", err)
	}

	statusByBranch := map[string]domain.SyncStepStatus{}
	for _, step := range result.Steps {
		statusByBranch[step.Branch] = step.Status
	}
	if statusByBranch["feat"] != domain.SyncStatusSynced {
		t.Fatalf("feat status = %q, want synced (%+v)", statusByBranch["feat"], result.Steps)
	}
	if statusByBranch["dev1"] != domain.SyncStatusSynced {
		t.Fatalf("dev1 status = %q, want synced (%+v)", statusByBranch["dev1"], result.Steps)
	}

	if !isAncestor(t, dir, "main", "feat") {
		t.Fatal("main should be an ancestor of feat after sync")
	}
	if !isAncestor(t, dir, "main", "dev1") {
		t.Fatal("main should be an ancestor of dev1 after sync")
	}
	if !isAncestor(t, dir, "feat", "dev1") {
		t.Fatal("feat should be an ancestor of dev1 after sync")
	}
}

// TestSyncSelectedBranchOnly verifies that targeting a single worktree rebases
// only it (onto a refreshed base) and leaves the sibling untouched.
func TestSyncSelectedBranchOnly(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	trees := t.TempDir()

	featPath := filepath.Join(trees, "feat")
	otherPath := filepath.Join(trees, "other")

	git(t, dir, "worktree", "add", "-b", "feat", featPath, "main")
	commitFile(t, featPath, "feat.txt", "feat work")
	git(t, dir, "worktree", "add", "-b", "other", otherPath, "main")
	commitFile(t, otherPath, "other.txt", "other work")

	// main diverges after both branches were created.
	commitFile(t, dir, "main.txt", "main moved")

	writeMeta(t, stateDir, "feat", "main")
	writeMeta(t, stateDir, "other", "main")

	result, err := Sync(t.Context(), SyncParams{
		ProjectDir:       dir,
		StateDir:         stateDir,
		BaseBranch:       "main",
		SelectedBranches: []string{"feat"},
	})
	if err != nil {
		t.Fatalf("Sync error: %v", err)
	}

	if len(result.Steps) != 1 || result.Steps[0].Branch != "feat" {
		t.Fatalf("expected a single feat step, got %+v", result.Steps)
	}
	if result.Steps[0].Status != domain.SyncStatusSynced {
		t.Fatalf("feat status = %q, want synced", result.Steps[0].Status)
	}

	if !isAncestor(t, dir, "main", "feat") {
		t.Fatal("feat should have been rebased onto the refreshed main")
	}
	if isAncestor(t, dir, "main", "other") {
		t.Fatal("other must NOT be rebased — it was not selected")
	}
}

// TestSyncSkipsDirtyAndDescendants verifies a dirty worktree is skipped along
// with its descendants, while independent branches still sync.
func TestSyncSkipsDirtyAndDescendants(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	trees := t.TempDir()

	featPath := filepath.Join(trees, "feat")
	dev1Path := filepath.Join(trees, "dev1")

	git(t, dir, "worktree", "add", "-b", "feat", featPath, "main")
	commitFile(t, featPath, "feat.txt", "feat work")
	git(t, dir, "worktree", "add", "-b", "dev1", dev1Path, "feat")
	commitFile(t, dev1Path, "dev1.txt", "dev1 work")
	commitFile(t, dir, "main.txt", "main moved")

	writeMeta(t, stateDir, "feat", "main")
	writeMeta(t, stateDir, "dev1", "feat")

	// Make feat dirty so it is skipped, which must also skip dev1.
	if err := os.WriteFile(filepath.Join(featPath, "feat.txt"), []byte("uncommitted"), 0o644); err != nil {
		t.Fatalf("dirty feat: %v", err)
	}

	result, err := Sync(t.Context(), SyncParams{
		ProjectDir: dir,
		StateDir:   stateDir,
		BaseBranch: "main",
	})
	if err != nil {
		t.Fatalf("Sync error: %v", err)
	}

	statusByBranch := map[string]domain.SyncStepStatus{}
	for _, step := range result.Steps {
		statusByBranch[step.Branch] = step.Status
	}
	if statusByBranch["feat"] != domain.SyncStatusSkippedDirty {
		t.Fatalf("feat status = %q, want skipped_dirty", statusByBranch["feat"])
	}
	if statusByBranch["dev1"] != domain.SyncStatusSkippedAncestor {
		t.Fatalf("dev1 status = %q, want skipped_ancestor", statusByBranch["dev1"])
	}
}

// TestSyncMissingParentRefIsError verifies that when the recorded parent branch
// does not exist as a ref, the failed behind-count is surfaced as an error
// rather than silently read as "0 → up_to_date" (regression for the
// error-swallowing revListCount).
func TestSyncMissingParentRefIsError(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	trees := t.TempDir()
	featPath := filepath.Join(trees, "feat")

	git(t, dir, "worktree", "add", "-b", "feat", featPath, "main")
	commitFile(t, featPath, "feat.txt", "feat work")

	// Record a parent branch that does not exist, so behind-counting fails.
	writeMeta(t, stateDir, "feat", "ghost-parent")

	result, err := Sync(t.Context(), SyncParams{ProjectDir: dir, StateDir: stateDir, BaseBranch: "main"})
	if err != nil {
		t.Fatalf("Sync error: %v", err)
	}
	if result.Steps[0].Status != domain.SyncStatusError {
		t.Fatalf("feat status = %q, want error (%+v)", result.Steps[0].Status, result.Steps)
	}
}
