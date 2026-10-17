package worktree

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTheFreeOrdinalIsTheSmallestNoWorktreeHoldsAfterTheMainCheckout(t *testing.T) {
	assert.Equal(t, 1, FreeOrdinal(nil))
	assert.Equal(t, 1, FreeOrdinal([]int{0, 0}))
	assert.Equal(t, 3, FreeOrdinal([]int{2, 1, 2}))
	assert.Equal(t, 2, FreeOrdinal([]int{1, 3}))
}
