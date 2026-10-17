package domain

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/stretchr/testify/assert"
)

func TestAnExtractConflictNamesWhatWasModifiedAndWhatAlreadyExists(t *testing.T) {
	refusal := ExtractConflict(ExtractConflictParams{Target: "feat/split", Conflicts: []worktree.Conflict{
		{Path: "a.go", Kind: worktree.ConflictModified},
		{Path: "b.go", Kind: worktree.ConflictModified},
		{Path: "new.txt", Kind: worktree.ConflictExists},
	}})

	assert.Equal(t, KindConflict, refusal.Kind)
	assert.Equal(t, CodeExtractConflict, refusal.Code)
	assert.Equal(t, `cannot apply the selected changes: a.go, b.go were also modified and new.txt already exists in "feat/split" — resolve them there, extract to another worktree, or re-run with --on-conflict resolve`, refusal.Message)
}

func TestAnExtractConflictOnOneFileSaysIt(t *testing.T) {
	refusal := ExtractConflict(ExtractConflictParams{Target: "main", Conflicts: []worktree.Conflict{{Path: "a.go", Kind: worktree.ConflictModified}}})

	assert.Equal(t, `cannot apply the selected changes: a.go was also modified in "main" — resolve it there, extract to another worktree, or re-run with --on-conflict resolve`, refusal.Message)
}

func TestABinaryCollisionIsRefusedWithoutOfferingToResolveIt(t *testing.T) {
	refusal := ExtractConflict(ExtractConflictParams{Target: "main", Conflicts: []worktree.Conflict{
		{Path: "a.go", Kind: worktree.ConflictModified},
		{Path: "logo.png", Kind: worktree.ConflictBinary},
	}})

	assert.Equal(t, `cannot apply the selected changes: logo.png already exists in "main" and is binary — conflict markers cannot be written, remove it there or extract to another worktree`, refusal.Message)
	assert.Equal(t, Params{"target": "main", "files": "logo.png"}, refusal.Params)
}
