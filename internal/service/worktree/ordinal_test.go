package worktree

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

type ordinalRepo struct {
	dir      string
	stateDir string
}

func newOrdinalRepo(t *testing.T) ordinalRepo {
	t.Helper()
	dir := gittest.InitRepo(t)
	return ordinalRepo{dir: dir, stateDir: filepath.Join(dir, ".git", "wtm")}
}

func (r ordinalRepo) addWorktree(t *testing.T, branch string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), rules.SanitizeBranchName(branch))
	git(t, r.dir, "worktree", "add", "-b", branch, path)
	return path
}

func (r ordinalRepo) ensure(t *testing.T, branch string) int {
	t.Helper()
	ordinal, err := EnsureOrdinal(WorktreeRef{
		ProjectDir: r.dir,
		StateDir:   r.stateDir,
		Branch:     branch,
	})
	if err != nil {
		t.Fatalf("EnsureOrdinal(%s): %v", branch, err)
	}
	return ordinal
}

func (r ordinalRepo) meta(t *testing.T, branch string) domain.WorktreeMetadata {
	t.Helper()
	meta, err := loadMetadata(r.stateDir, branch)
	if err != nil {
		t.Fatalf("loadMetadata(%s): %v", branch, err)
	}
	return meta
}

func TestEnsureOrdinalMainKeepsZeroAndWritesNothing(t *testing.T) {
	repo := newOrdinalRepo(t)

	if got := repo.ensure(t, "main"); got != domain.MainWorktreeOrdinal {
		t.Errorf("main ordinal = %d, want %d", got, domain.MainWorktreeOrdinal)
	}

	metaPath := filepath.Join(rules.WorktreeMetaDir(repo.stateDir, "main"), domain.MetaFileName)
	if _, err := os.Stat(metaPath); err == nil {
		t.Error("main worktree got a meta.json, want none")
	}
}

func TestEnsureOrdinalAllocatesAndPersists(t *testing.T) {
	repo := newOrdinalRepo(t)
	repo.addWorktree(t, "feat/one")

	if got := repo.ensure(t, "feat/one"); got != 1 {
		t.Fatalf("first linked worktree = %d, want 1", got)
	}
	if got := repo.meta(t, "feat/one").Ordinal; got != 1 {
		t.Errorf("persisted ordinal = %d, want 1", got)
	}

	// Idempotent: asking again returns the number already recorded.
	if got := repo.ensure(t, "feat/one"); got != 1 {
		t.Errorf("second call = %d, want 1", got)
	}
}

func TestEnsureOrdinalGivesEachWorktreeItsOwn(t *testing.T) {
	repo := newOrdinalRepo(t)
	repo.addWorktree(t, "feat/one")
	repo.addWorktree(t, "feat/two")

	first := repo.ensure(t, "feat/one")
	second := repo.ensure(t, "feat/two")

	if first == second {
		t.Fatalf("both worktrees got ordinal %d", first)
	}
	if first != 1 || second != 2 {
		t.Errorf("ordinals = %d/%d, want 1/2", first, second)
	}
}

func TestEnsureOrdinalRecyclesRemovedWorktreeNumber(t *testing.T) {
	repo := newOrdinalRepo(t)
	gone := repo.addWorktree(t, "feat/gone")
	repo.addWorktree(t, "feat/two")

	repo.ensure(t, "feat/gone")
	if got := repo.ensure(t, "feat/two"); got != 2 {
		t.Fatalf("feat/two = %d, want 2", got)
	}

	// The worktree goes away but its meta.json stays behind, as it does for any
	// removal wtm did not drive. A stale meta must not reserve a number.
	git(t, repo.dir, "worktree", "remove", "--force", gone)
	repo.addWorktree(t, "feat/three")

	if got := repo.ensure(t, "feat/three"); got != 1 {
		t.Errorf("feat/three = %d, want 1 (the freed number)", got)
	}
}

func TestEnsureOrdinalBackfillsWithoutLosingMetadata(t *testing.T) {
	repo := newOrdinalRepo(t)
	repo.addWorktree(t, "feat/external")
	writeMeta(t, repo.stateDir, "feat/external", "main")

	before := repo.meta(t, "feat/external")
	if before.Ordinal != 0 {
		t.Fatalf("fixture already has ordinal %d", before.Ordinal)
	}

	if got := repo.ensure(t, "feat/external"); got != 1 {
		t.Fatalf("backfilled ordinal = %d, want 1", got)
	}

	after := repo.meta(t, "feat/external")
	if after.SourceBranch != before.SourceBranch {
		t.Errorf("source_branch = %q, want %q", after.SourceBranch, before.SourceBranch)
	}
	if after.CreatedAt != before.CreatedAt {
		t.Errorf("created_at = %q, want %q", after.CreatedAt, before.CreatedAt)
	}
}

func TestEnsureOrdinalRepairsDuplicate(t *testing.T) {
	cases := []struct {
		name    string
		askedAt string
	}{
		// The outcome must not depend on who asks: whichever worktree runs first,
		// the lowest branch keeps the number and the other moves. Without that,
		// both sides re-allocate, land on the same free number and collide again.
		{"le perdant demande en premier", "feat/one"},
		{"le gagnant demande en premier", "feat/clone"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := newOrdinalRepo(t)
			repo.addWorktree(t, "feat/one")
			repo.addWorktree(t, "feat/clone")

			// A meta.json copied by hand, or a lock that could not be taken: two
			// live worktrees claiming the same number.
			for _, branch := range []string{"feat/one", "feat/clone"} {
				duplicate := domain.WorktreeMetadata{SourceBranch: "main", Ordinal: 1}
				if err := writeMetadata(rules.WorktreeMetaDir(repo.stateDir, branch), duplicate); err != nil {
					t.Fatalf("write duplicate: %v", err)
				}
			}

			repo.ensure(t, c.askedAt)

			if got := repo.ensure(t, "feat/clone"); got != 1 {
				t.Errorf("feat/clone = %d, want 1 (its branch sorts first, it keeps the number)", got)
			}
			if got := repo.ensure(t, "feat/one"); got == 1 {
				t.Error("feat/one kept the contested number, want it moved")
			}
		})
	}
}

// Asking twice in a row must not move anything: a repair that re-allocated on
// every run would walk a worktree's ports out from under its running jobs.
func TestEnsureOrdinalConvergesAfterRepair(t *testing.T) {
	repo := newOrdinalRepo(t)
	repo.addWorktree(t, "feat/one")
	repo.addWorktree(t, "feat/clone")

	for _, branch := range []string{"feat/one", "feat/clone"} {
		if err := writeMetadata(rules.WorktreeMetaDir(repo.stateDir, branch), domain.WorktreeMetadata{Ordinal: 1}); err != nil {
			t.Fatalf("write duplicate: %v", err)
		}
	}

	repaired := repo.ensure(t, "feat/one")
	for range 3 {
		if got := repo.ensure(t, "feat/one"); got != repaired {
			t.Fatalf("ordinal moved again: %d then %d", repaired, got)
		}
		if got := repo.ensure(t, "feat/clone"); got != 1 {
			t.Fatalf("feat/clone moved to %d, want a stable 1", got)
		}
	}
}

// A worktree that merely ran a job has a meta.json holding its ordinal and
// nothing else. It is still external: relocate must keep offering to adopt it,
// or it would never get the parent sync needs.
func TestEnsureOrdinalLeavesWorktreeExternal(t *testing.T) {
	repo := newOrdinalRepo(t)
	repo.addWorktree(t, "feat/external")

	if isManaged(repo.stateDir, "feat/external") {
		t.Fatal("fixture already looks managed")
	}

	repo.ensure(t, "feat/external")

	if isManaged(repo.stateDir, "feat/external") {
		t.Error("allocating an ordinal made an external worktree look wtm-managed")
	}
}

func TestCreatedWorktreeIsManaged(t *testing.T) {
	repo := newOrdinalRepo(t)
	repo.addWorktree(t, "feat/adopted")

	if err := writeMetadata(rules.WorktreeMetaDir(repo.stateDir, "feat/adopted"), domain.WorktreeMetadata{
		SourceBranch: "main",
		CreatedAt:    "2026-08-21T00:00:00Z",
	}); err != nil {
		t.Fatalf("write meta: %v", err)
	}

	if !isManaged(repo.stateDir, "feat/adopted") {
		t.Error("a worktree wtm created or adopted is not seen as managed")
	}
}

// The whole point of the ticket: worktrees numbered at the same instant must
// still get distinct numbers. The list of worktrees is read inside the lock, so
// an allocation cannot be computed against a repository state another process
// has already moved past.
func TestEnsureOrdinalNeverCollidesUnderConcurrency(t *testing.T) {
	repo := newOrdinalRepo(t)

	branches := []string{"feat/a", "feat/b", "feat/c", "feat/d", "feat/e", "feat/f"}
	for _, branch := range branches {
		repo.addWorktree(t, branch)
	}

	var wg sync.WaitGroup
	got := make([]int, len(branches))
	for i, branch := range branches {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ordinal, err := EnsureOrdinal(WorktreeRef{
				ProjectDir: repo.dir,
				StateDir:   repo.stateDir,
				Branch:     branch,
			})
			if err != nil {
				t.Errorf("EnsureOrdinal(%s): %v", branch, err)
				return
			}
			got[i] = ordinal
		}()
	}
	wg.Wait()

	seen := make(map[int]string, len(branches))
	for i, ordinal := range got {
		if owner, taken := seen[ordinal]; taken {
			t.Errorf("%s and %s both got ordinal %d", owner, branches[i], ordinal)
		}
		seen[ordinal] = branches[i]
	}
}

// An unreadable meta.json means "holds an unknown number", not "holds nothing".
// Skipping it would hand its number to the worktree asking.
func TestEnsureOrdinalRefusesWhenAnotherOrdinalIsUnreadable(t *testing.T) {
	repo := newOrdinalRepo(t)
	repo.addWorktree(t, "feat/one")
	repo.addWorktree(t, "feat/two")
	repo.ensure(t, "feat/one")

	metaPath := filepath.Join(rules.WorktreeMetaDir(repo.stateDir, "feat/one"), domain.MetaFileName)
	if err := os.WriteFile(metaPath, []byte("{ truncated"), 0o644); err != nil {
		t.Fatalf("corrupt meta: %v", err)
	}

	_, err := EnsureOrdinal(WorktreeRef{ProjectDir: repo.dir, StateDir: repo.stateDir, Branch: "feat/two"})
	if !errors.Is(err, domain.ErrOrdinalUnreadable) {
		t.Errorf("error = %v, want %v", err, domain.ErrOrdinalUnreadable)
	}
}

func TestEnsureOrdinalRefusesAnIncompleteReference(t *testing.T) {
	repo := newOrdinalRepo(t)

	cases := []struct {
		name string
		ref  WorktreeRef
	}{
		{"sans project dir", WorktreeRef{StateDir: repo.stateDir, Branch: "feat/x"}},
		{"sans state dir", WorktreeRef{ProjectDir: repo.dir, Branch: "feat/x"}},
		{"sans branche", WorktreeRef{ProjectDir: repo.dir, StateDir: repo.stateDir}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := EnsureOrdinal(c.ref); !errors.Is(err, domain.ErrOrdinalRefIncomplete) {
				t.Errorf("error = %v, want %v", err, domain.ErrOrdinalRefIncomplete)
			}
		})
	}
}

// A worktree git cannot name has no metadata to look up. It must not break the
// numbering of the worktrees around it.
func TestEnsureOrdinalToleratesDetachedHead(t *testing.T) {
	repo := newOrdinalRepo(t)
	detached := repo.addWorktree(t, "feat/detached")
	repo.addWorktree(t, "feat/live")
	repo.ensure(t, "feat/detached")

	git(t, detached, "checkout", "--detach")

	if got := repo.ensure(t, "feat/live"); got == 0 {
		t.Error("feat/live got the main checkout's ordinal")
	}
}
