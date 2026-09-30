package rules

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
)

// ValidateReparentParams holds inputs for validating a single parent change.
type ValidateReparentParams struct {
	Nodes      []domain.WorktreeNode
	Branch     string
	NewParent  string
	BaseBranch string
}

// ValidateReparentBatchParams holds inputs for validating a batch parent change.
type ValidateReparentBatchParams struct {
	Nodes      []domain.WorktreeNode
	Branches   []string
	NewParent  string
	BaseBranch string
}

// ValidateReparentBatch checks that pointing every branch in Branches at NewParent is
// legal: each must be a managed worktree, none may become its own parent, and the
// resulting parent graph must stay acyclic once ALL edges are changed together.
// Applying every edge before the single BuildSyncPlan call catches cycles that arise
// only from the combined change. Cycle detection reuses BuildSyncPlan so there is a
// single source of truth for the topological invariant.
func ValidateReparentBatch(params ValidateReparentBatchParams) error {
	targets := make(map[string]bool, len(params.Branches))
	for _, branch := range params.Branches {
		if params.NewParent == branch {
			return fmt.Errorf("%w: %s", domain.ErrReparentSelf, branch)
		}
		targets[branch] = true
	}

	found := make(map[string]bool, len(params.Branches))
	mutated := make([]domain.WorktreeNode, len(params.Nodes))
	for i, node := range params.Nodes {
		if targets[node.Branch] {
			node.SourceBranch = params.NewParent
			found[node.Branch] = true
		}
		mutated[i] = node
	}
	for _, branch := range params.Branches {
		if !found[branch] {
			return fmt.Errorf("%w: %s", domain.ErrWorktreeNotFound, branch)
		}
	}

	if _, err := BuildSyncPlan(BuildSyncPlanParams{Nodes: mutated, BaseBranch: params.BaseBranch}); err != nil {
		return err
	}
	return nil
}

// UniqueStrings returns the elements of in with duplicates removed, preserving
// first-seen order. Used to collapse repeated worktree arguments so a batch
// operation acts on (and reports) each branch once.
func UniqueStrings(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// ChildrenOfParams holds inputs for ChildrenOf.
type ChildrenOfParams struct {
	Nodes  []domain.WorktreeNode
	Branch string
}

// ChildrenOf returns the managed worktrees whose recorded parent is Branch.
func ChildrenOf(params ChildrenOfParams) []domain.WorktreeNode {
	children := make([]domain.WorktreeNode, 0)
	for _, node := range params.Nodes {
		if node.SourceBranch == params.Branch {
			children = append(children, node)
		}
	}
	return children
}

// ReparentExclusionsParams holds inputs for ReparentExclusions.
type ReparentExclusionsParams struct {
	Nodes      []domain.WorktreeNode
	Branches   []string
	BaseBranch string
}

// ReparentExclusions names the branches a selection may not be rebased onto: the
// selected worktrees themselves, and anything whose parent chain runs through
// them. Only a member of the forest can be either, so walking the nodes is
// complete — a branch outside it can never close a cycle.
//
// It is stated as an exclusion rather than as an allowed list so a picker can
// apply it over a candidate list that is still being refreshed underneath.
func ReparentExclusions(params ReparentExclusionsParams) []string {
	if len(params.Branches) == 0 {
		return nil
	}
	excluded := make([]string, 0, len(params.Nodes))
	for _, node := range params.Nodes {
		err := ValidateReparentBatch(ValidateReparentBatchParams{
			Nodes:      params.Nodes,
			Branches:   params.Branches,
			NewParent:  node.Branch,
			BaseBranch: params.BaseBranch,
		})
		if err != nil {
			excluded = append(excluded, node.Branch)
		}
	}
	return excluded
}
