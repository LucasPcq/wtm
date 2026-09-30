package rules

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func reparentNodes() []domain.WorktreeNode {
	return []domain.WorktreeNode{
		{Branch: "main", IsMain: true},
		{Branch: "feat", SourceBranch: "main"},
		{Branch: "dev/a", SourceBranch: "feat"},
		{Branch: "dev/b", SourceBranch: "dev/a"},
	}
}

func TestValidateReparentBatchValid(t *testing.T) {
	// Flatten the stack: both dev worktrees move onto main in one pass.
	err := ValidateReparentBatch(ValidateReparentBatchParams{
		Nodes:      reparentNodes(),
		Branches:   []string{"dev/a", "dev/b"},
		NewParent:  "main",
		BaseBranch: "main",
	})
	if err != nil {
		t.Fatalf("expected valid batch reparent, got %v", err)
	}
}

func TestValidateReparentBatchRejectsCycle(t *testing.T) {
	// Moving both feat and dev/a onto dev/b, while dev/b still points at dev/a,
	// closes the loop dev/a → dev/b → dev/a. The batch must validate the combined
	// graph in one pass and reject it.
	err := ValidateReparentBatch(ValidateReparentBatchParams{
		Nodes:      reparentNodes(),
		Branches:   []string{"feat", "dev/a"},
		NewParent:  "dev/b",
		BaseBranch: "main",
	})
	if err == nil {
		t.Fatalf("expected a cycle error across the batch, got nil")
	}
}

func TestValidateReparentBatchRejectsSelfParent(t *testing.T) {
	err := ValidateReparentBatch(ValidateReparentBatchParams{
		Nodes:      reparentNodes(),
		Branches:   []string{"dev/a", "feat"},
		NewParent:  "feat",
		BaseBranch: "main",
	})
	if !errors.Is(err, domain.ErrReparentSelf) {
		t.Fatalf("expected ErrReparentSelf when a listed branch equals the new parent, got %v", err)
	}
}

func TestValidateReparentBatchRejectsUnknownBranch(t *testing.T) {
	err := ValidateReparentBatch(ValidateReparentBatchParams{
		Nodes:      reparentNodes(),
		Branches:   []string{"dev/a", "ghost"},
		NewParent:  "main",
		BaseBranch: "main",
	})
	if !errors.Is(err, domain.ErrWorktreeNotFound) {
		t.Fatalf("expected ErrWorktreeNotFound for an unmanaged branch, got %v", err)
	}
}

func TestChildrenOf(t *testing.T) {
	children := ChildrenOf(ChildrenOfParams{Nodes: reparentNodes(), Branch: "feat"})
	if len(children) != 1 || children[0].Branch != "dev/a" {
		t.Fatalf("expected [dev/a], got %+v", children)
	}

	if got := ChildrenOf(ChildrenOfParams{Nodes: reparentNodes(), Branch: "dev/b"}); len(got) != 0 {
		t.Fatalf("expected no children for leaf dev/b, got %+v", got)
	}
}

func exclusionSet(t *testing.T, branches []string) map[string]bool {
	t.Helper()
	// main ← feat ← dev-a ← dev-b
	nodes := []domain.WorktreeNode{
		{Branch: "main", IsMain: true},
		{Branch: "feat", SourceBranch: "main"},
		{Branch: "dev-a", SourceBranch: "feat"},
		{Branch: "dev-b", SourceBranch: "dev-a"},
	}
	set := make(map[string]bool)
	for _, name := range ReparentExclusions(ReparentExclusionsParams{
		Nodes:      nodes,
		Branches:   branches,
		BaseBranch: "main",
	}) {
		set[name] = true
	}
	return set
}

func TestReparentExclusionsDropTheSelectionAndItsDescendants(t *testing.T) {
	excluded := exclusionSet(t, []string{"feat"})

	if excluded["main"] {
		t.Error("main should stay a legal parent for feat")
	}
	if !excluded["feat"] {
		t.Error("a worktree cannot be offered as its own parent")
	}
	// feat is an ancestor of both, so either would close a cycle.
	if !excluded["dev-a"] || !excluded["dev-b"] {
		t.Errorf("descendants of the selection must be excluded: %v", excluded)
	}
}

// The hole the picker regression went through: with nothing selected there is
// nothing to exclude, and every branch is legal — including the ones that stop
// being legal the moment the user checks a box.
func TestReparentExclusionsAreEmptyWithNoSelection(t *testing.T) {
	if excluded := exclusionSet(t, nil); len(excluded) != 0 {
		t.Errorf("excluded = %v, want nothing before a selection exists", excluded)
	}
}

func TestReparentExclusionsCoverEverySelectedWorktree(t *testing.T) {
	excluded := exclusionSet(t, []string{"feat", "dev-b"})

	for _, name := range []string{"feat", "dev-a", "dev-b"} {
		if !excluded[name] {
			t.Errorf("%s must be excluded for the batch, got %v", name, excluded)
		}
	}
	if excluded["main"] {
		t.Error("main is still a legal parent for the whole batch")
	}
}

// A branch outside the forest cannot be a worktree's descendant, so it is never
// excluded — which is what keeps origin/* selectable.
func TestReparentExclusionsIgnoreBranchesOutsideTheForest(t *testing.T) {
	if excluded := exclusionSet(t, []string{"feat"}); excluded["origin/staging"] {
		t.Error("a branch with no worktree can never close a cycle")
	}
}
