package target_test

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/flow/run/target"
)

func TestNamedBranchPrefersTheBranchThePositionalNamed(t *testing.T) {
	dir := t.TempDir()
	got := target.NamedBranch(target.NamedBranchParams{
		Named: []target.Resolved{{Dir: "/elsewhere", Branch: "other"}, {Dir: dir, Branch: "feat/x"}},
		Dir:   dir,
	})
	if got != "feat/x" {
		t.Fatalf("NamedBranch = %q, want feat/x", got)
	}
}

func TestNamedBranchAsksGitForAWorktreeNobodyNamed(t *testing.T) {
	got := target.NamedBranch(target.NamedBranchParams{
		Named: []target.Resolved{{Dir: "/elsewhere", Branch: "other"}},
		Dir:   t.TempDir(),
	})
	if got != "" {
		t.Fatalf("NamedBranch outside a repository = %q, want empty", got)
	}
}
