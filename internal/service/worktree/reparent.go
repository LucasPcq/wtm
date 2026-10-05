package worktree

import (
	"context"
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/rules"
)

// ReparentBatch changes the recorded parent (source_branch) of one or more worktrees
// to the same new parent in a single pass. It only rewrites metadata — the actual
// rebase happens on the next `wtm sync`. The new parent must exist as a local branch
// or an origin remote-tracking branch (origin/x), and the combined change must keep
// the parent graph acyclic. A single-element Branches is the ordinary one-worktree
// reparent.
func ReparentBatch(ctx context.Context, params domain.ReparentBatchParams) ([]domain.ReparentResult, error) {
	nodes, err := buildNodes(ctx, params.ProjectDir, params.StateDir)
	if err != nil {
		return nil, err
	}

	// Collapse repeated arguments (`wtm reparent feat feat`) so each worktree is
	// validated and rewritten once — and reported once in the results.
	branches := rules.UniqueStrings(params.Branches)

	for _, branch := range branches {
		if !isManaged(params.StateDir, branch) {
			return nil, fmt.Errorf("%w: %s", domain.ErrWorktreeNotFound, branch)
		}
	}

	if !infra.BranchOrRemoteExists(ctx, infra.BranchOrRemoteExistsParams{
		ProjectDir: params.ProjectDir,
		Ref:        params.NewParent,
	}) {
		return nil, fmt.Errorf("%w: %s", domain.ErrBranchNotFound, params.NewParent)
	}

	if err := rules.ValidateReparentBatch(rules.ValidateReparentBatchParams{
		Nodes:      nodes,
		Branches:   branches,
		NewParent:  params.NewParent,
		BaseBranch: params.BaseBranch,
	}); err != nil {
		return nil, err
	}

	results := make([]domain.ReparentResult, 0, len(branches))
	for _, branch := range branches {
		res, err := setSourceBranch(setSourceBranchParams{
			StateDir:  params.StateDir,
			Branch:    branch,
			NewParent: params.NewParent,
		})
		if err != nil {
			// Partial failure has no rollback: branches already rewritten in this loop
			// keep their new parent, and results reports what succeeded. All validation
			// (managed, parent existence, acyclicity) runs before the loop, so only an
			// I/O write fault reaches here.
			return results, err
		}
		results = append(results, res)
	}
	return results, nil
}

// ApplyReparentsParams holds inputs for ApplyReparents.
type ApplyReparentsParams struct {
	Reparents []domain.ReparentResult
	StateDir  string
}

// ApplyReparents rewrites each listed child's metadata to point at its NewParent.
func ApplyReparents(params ApplyReparentsParams) ([]domain.ReparentResult, error) {
	applied := make([]domain.ReparentResult, 0, len(params.Reparents))
	for _, r := range params.Reparents {
		res, err := setSourceBranch(setSourceBranchParams{
			StateDir:  params.StateDir,
			Branch:    r.Branch,
			NewParent: r.NewParent,
		})
		if err != nil {
			return applied, err
		}
		applied = append(applied, res)
	}
	return applied, nil
}

// setSourceBranchParams holds inputs for setSourceBranch.
type setSourceBranchParams struct {
	StateDir  string
	Branch    string
	NewParent string
}

// setSourceBranch updates only the SourceBranch field of a worktree's metadata,
// preserving CreatedAt and EnvStrategy.
func setSourceBranch(params setSourceBranchParams) (domain.ReparentResult, error) {
	meta, err := loadMetadata(params.StateDir, params.Branch)
	if err != nil {
		return domain.ReparentResult{}, fmt.Errorf("read metadata for %s: %w", params.Branch, err)
	}

	old := meta.SourceBranch
	meta.SourceBranch = params.NewParent

	if err := writeMetadata(rules.WorktreeMetaDir(params.StateDir, params.Branch), meta); err != nil {
		return domain.ReparentResult{}, err
	}

	return domain.ReparentResult{Branch: params.Branch, OldParent: old, NewParent: params.NewParent}, nil
}
