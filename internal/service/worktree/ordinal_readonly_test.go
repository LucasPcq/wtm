package worktree

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestReadingTheEnvironmentNeverNumbersAWorktree(t *testing.T) {
	repo := newOrdinalRepo(t)
	repo.addWorktree(t, "feat/x")
	recordIsolation(t, repo, "feat/x", domain.IsolationIsolated)
	ref := WorktreeRef{ProjectDir: repo.dir, StateDir: repo.stateDir, Branch: "feat/x"}

	if _, err := BranchEnv(t.Context(), ref); !errors.Is(err, domain.ErrOrdinalUnallocated) {
		t.Fatalf("BranchEnv = %v, want ErrOrdinalUnallocated", err)
	}
	if got := repo.meta(t, "feat/x").Ordinal; got != 0 {
		t.Fatalf("reading the environment allocated ordinal %d", got)
	}
}

func TestEnsureOrdinalSaysWhetherItAllocated(t *testing.T) {
	repo := newOrdinalRepo(t)
	repo.addWorktree(t, "feat/x")
	ref := WorktreeRef{ProjectDir: repo.dir, StateDir: repo.stateDir, Branch: "feat/x"}

	first, err := EnsureOrdinal(t.Context(), ref)
	if err != nil {
		t.Fatal(err)
	}
	second, err := EnsureOrdinal(t.Context(), ref)
	if err != nil {
		t.Fatal(err)
	}
	if first != (OrdinalClaim{Ordinal: 1, Allocated: true}) || second != (OrdinalClaim{Ordinal: 1, Allocated: false}) {
		t.Fatalf("first = %+v, second = %+v", first, second)
	}
}

func TestHookEnvIsPendingOnlyWhenHooksWouldReadAnUnnumberedWorktree(t *testing.T) {
	repo := newOrdinalRepo(t)
	repo.addWorktree(t, "feat/x")
	ref := WorktreeRef{ProjectDir: repo.dir, StateDir: repo.stateDir, Branch: "feat/x"}

	if HookEnvPending(t.Context(), ref) {
		t.Fatal("pending with no run.toml")
	}
	writeRunConfig(t, repo.stateDir, composeJobConfig)
	recordIsolation(t, repo, "feat/x", domain.IsolationIsolated)
	if !HookEnvPending(t.Context(), ref) {
		t.Fatal("not pending for an unnumbered worktree whose hooks read the run env")
	}
	repo.ensure(t, "feat/x")
	if HookEnvPending(t.Context(), ref) {
		t.Fatal("still pending once numbered")
	}
}
