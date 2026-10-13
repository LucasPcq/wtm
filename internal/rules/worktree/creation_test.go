package worktree_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules/worktree"
)

func TestAFreeBranchAndFolderAreAdded(t *testing.T) {
	assert.Equal(t, worktree.Add{}, worktree.CreationOf(worktree.CreationParams{Branch: "feat/x", Path: "/wt/feat-x"}))
}

func TestABranchOutElsewhereIsUnchangedOnlyUnderIfNotExists(t *testing.T) {
	params := worktree.CreationParams{Branch: "feat/x", Path: "/wt/feat-x", CheckedOutAt: "/elsewhere"}
	assert.Equal(t, worktree.CheckedOutElsewhere{Path: "/elsewhere"}, worktree.CreationOf(params))

	params.IfNotExists = true
	assert.Equal(t, worktree.Unchanged{Path: "/elsewhere"}, worktree.CreationOf(params))
}

func TestAFolderHoldingTheSameBranchIsUnchangedOnlyUnderIfNotExists(t *testing.T) {
	params := worktree.CreationParams{Branch: "feat/x", Path: "/wt/feat-x", PathTaken: true, Occupant: "feat/x"}
	assert.Equal(t, worktree.PathTaken{Occupant: "feat/x"}, worktree.CreationOf(params))

	params.IfNotExists = true
	assert.Equal(t, worktree.Unchanged{Path: "/wt/feat-x"}, worktree.CreationOf(params))
}

func TestAFolderHoldingAnotherBranchIsNeverUnchanged(t *testing.T) {
	params := worktree.CreationParams{Branch: "feat/x", Path: "/wt/feat-x", PathTaken: true, Occupant: "feat-x", IfNotExists: true}
	assert.Equal(t, worktree.PathTaken{Occupant: "feat-x"}, worktree.CreationOf(params))
}

func TestAFolderNoWorktreeHoldsIsTaken(t *testing.T) {
	params := worktree.CreationParams{Branch: "feat/x", Path: "/wt/feat-x", PathTaken: true, IfNotExists: true}
	assert.Equal(t, worktree.PathTaken{}, worktree.CreationOf(params))
}

func TestAWorktreeIsManagedOnceWtmRecordedItsCreation(t *testing.T) {
	assert.False(t, worktree.Managed(domain.WorktreeMetadata{}))
	assert.False(t, worktree.Managed(domain.WorktreeMetadata{Ordinal: 3}))
	assert.True(t, worktree.Managed(domain.WorktreeMetadata{CreatedAt: "2026-10-12T10:00:00Z"}))
}
