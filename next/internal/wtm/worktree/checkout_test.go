package worktree

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/wtm/branch"
	"github.com/stretchr/testify/assert"
)

func TestOnlyANewOrATrackingCheckoutCreatesItsBranch(t *testing.T) {
	created, ok := CreatedBranch(NewBranch{Name: "topic", From: branch.LocalBranch{Name: "main"}})
	assert.True(t, ok)
	assert.Equal(t, "topic", created)

	created, ok = CreatedBranch(TrackingBranch{Name: "remote-only"})
	assert.True(t, ok)
	assert.Equal(t, "remote-only", created)

	_, ok = CreatedBranch(ExistingBranch{Name: "feature"})
	assert.False(t, ok)
}
