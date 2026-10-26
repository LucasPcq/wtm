package form_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

func TestEveryBlockerHasItsWords(t *testing.T) {
	for _, blocker := range worktree.Blockers() {
		assert.NotEmpty(t, form.Blockers([]worktree.Blocker{blocker}), blocker)
	}
	assert.Equal(t, "locked, uncommitted changes", form.Blockers([]worktree.Blocker{worktree.BlockerLocked, worktree.BlockerDirty}))
}
