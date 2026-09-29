package worktree

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestIsolationOfAWorktreeWithNoRecordIsIsolated(t *testing.T) {
	repo := newOrdinalRepo(t)
	repo.addWorktree(t, "feat/x")

	ref := WorktreeRef{ProjectDir: repo.dir, StateDir: repo.stateDir, Branch: "feat/x"}
	if got := IsolationOf(ref); got != domain.IsolationIsolated {
		t.Errorf("IsolationOf = %q, want %q", got, domain.IsolationIsolated)
	}
}

// The record is read-modify-write: switching the choice must not cost the
// worktree its ordinal, which every port and name is derived from.
func TestSetIsolationKeepsTheRestOfTheRecord(t *testing.T) {
	repo := newOrdinalRepo(t)
	repo.addWorktree(t, "feat/x")
	ordinal := repo.ensure(t, "feat/x")

	ref := WorktreeRef{ProjectDir: repo.dir, StateDir: repo.stateDir, Branch: "feat/x"}
	if err := SetIsolation(SetIsolationParams{Ref: ref, Isolation: domain.IsolationVerbatim}); err != nil {
		t.Fatalf("SetIsolation: %v", err)
	}

	if got := IsolationOf(ref); got != domain.IsolationVerbatim {
		t.Errorf("IsolationOf = %q, want %q", got, domain.IsolationVerbatim)
	}
	if got := repo.meta(t, "feat/x").Ordinal; got != ordinal {
		t.Errorf("ordinal = %d after the switch, want %d", got, ordinal)
	}
}

func TestSetIsolationRefusesAVerbatimMain(t *testing.T) {
	repo := newOrdinalRepo(t)
	ref := WorktreeRef{ProjectDir: repo.dir, StateDir: repo.stateDir, Branch: "main"}

	err := SetIsolation(SetIsolationParams{Ref: ref, Isolation: domain.IsolationVerbatim})
	if !errors.Is(err, domain.ErrIsolationMain) {
		t.Errorf("err = %v, want %v", err, domain.ErrIsolationMain)
	}
	if err := SetIsolation(SetIsolationParams{Ref: ref, Isolation: domain.IsolationIsolated}); err != nil {
		t.Errorf("isolating main is what it already is, want no error: %v", err)
	}
}

func TestCreateRecordsTheIsolationChosen(t *testing.T) {
	repo := newOrdinalRepo(t)
	var cfg domain.Config
	cfg.Project.Worktrees.BasePath = t.TempDir()

	if _, err := Create(domain.CreateParams{
		ProjectDir: repo.dir,
		StateDir:   repo.stateDir,
		Branch:     "feat/x",
		FromBranch: "main",
		Config:     cfg,
		SkipHooks:  true,
		Isolation:  domain.IsolationVerbatim,
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if got := repo.meta(t, "feat/x").Isolation; got != domain.IsolationVerbatim {
		t.Errorf("recorded isolation = %q, want %q", got, domain.IsolationVerbatim)
	}
}
