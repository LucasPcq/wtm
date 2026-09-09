package worktree_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

// A shared job runs in the main checkout, so resolving it has to work from any
// worktree — including one that is not it.
func TestMainCheckoutFoundFromALinkedWorktree(t *testing.T) {
	repo := gittest.InitRepo(t)
	linked := filepath.Join(t.TempDir(), "feat-x")
	gittest.Git(t, repo, "worktree", "add", "-b", "feat-x", linked)

	got, err := worktree.MainCheckout(worktree.MainCheckoutParams{ProjectDir: linked})
	if err != nil {
		t.Fatalf("MainCheckout: %v", err)
	}

	want, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	if got != want {
		t.Errorf("MainCheckout = %q, want %q", got, want)
	}
}

func TestMainCheckoutFoundFromTheMainCheckoutItself(t *testing.T) {
	repo := gittest.InitRepo(t)
	got, err := worktree.MainCheckout(worktree.MainCheckoutParams{ProjectDir: repo})
	if err != nil {
		t.Fatalf("MainCheckout: %v", err)
	}
	want, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	if got != want {
		t.Errorf("MainCheckout = %q, want %q", got, want)
	}
}

func TestMainCheckoutErrorsOutsideARepository(t *testing.T) {
	_, err := worktree.MainCheckout(worktree.MainCheckoutParams{ProjectDir: t.TempDir()})
	if err == nil {
		t.Fatal("MainCheckout outside a repository = nil error")
	}
	if !errors.Is(err, domain.ErrNoMainCheckout) && err == nil {
		t.Errorf("err = %v", err)
	}
}

// The metadata is the only durable record that a worktree holds a tenant: a
// claim on a shared service goes with a `run stop`, and without this a clean
// would either give back a tenant that was never created or leak one that was.
func TestRecordTenantsIsAdditiveAndIdempotent(t *testing.T) {
	repo := gittest.InitRepo(t)
	stateDir := filepath.Join(repo, ".git", "wtm")
	metaDir := filepath.Join(stateDir, "worktrees", "feat-x")
	if err := os.MkdirAll(metaDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(metaDir, domain.MetaFileName), []byte(`{"source_branch":"main","ordinal":2}`), 0o644); err != nil {
		t.Fatalf("write meta: %v", err)
	}

	params := worktree.ParentBranchParams{StateDir: stateDir, Branch: "feat-x"}
	if got := worktree.TenantsOf(params); len(got) != 0 {
		t.Fatalf("tenants = %v, want none", got)
	}

	record := func(jobs ...string) {
		t.Helper()
		if err := worktree.RecordTenants(worktree.RecordTenantsParams{
			StateDir: stateDir, Branch: "feat-x", Jobs: jobs,
		}); err != nil {
			t.Fatalf("RecordTenants: %v", err)
		}
	}

	record("db")
	record("db", "keycloak")
	got := worktree.TenantsOf(params)
	if len(got) != 2 || got[0] != "db" || got[1] != "keycloak" {
		t.Errorf("tenants = %v, want [db keycloak] recorded once each", got)
	}

	// The rest of the file survives: the ordinal is what every port derives from.
	meta, ok := worktree.Metadata(params)
	if !ok || meta.Ordinal != 2 || meta.SourceBranch != "main" {
		t.Errorf("metadata = %+v, want the other fields untouched", meta)
	}
}

// The main checkout has no meta.json, and recording must not create one.
func TestRecordTenantsCreatesNoMetadata(t *testing.T) {
	stateDir := t.TempDir()
	if err := worktree.RecordTenants(worktree.RecordTenantsParams{
		StateDir: stateDir, Branch: "main", Jobs: []string{"db"},
	}); err != nil {
		t.Fatalf("RecordTenants: %v", err)
	}
	if got := worktree.TenantsOf(worktree.ParentBranchParams{StateDir: stateDir, Branch: "main"}); len(got) != 0 {
		t.Errorf("tenants = %v, want none", got)
	}
}
