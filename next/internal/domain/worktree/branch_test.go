package worktree

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestABranchStandsAgainstOriginByItsCountsOnBothSides(t *testing.T) {
	assert.Equal(t, UpToDate, AheadBehind{}.Divergence())
	assert.Equal(t, Behind, AheadBehind{Behind: 2}.Divergence())
	assert.Equal(t, Ahead, AheadBehind{Ahead: 1}.Divergence())
	assert.Equal(t, Diverged, AheadBehind{Ahead: 1, Behind: 2}.Divergence())
}

func TestCountsReadAsAheadThenBehind(t *testing.T) {
	assert.Equal(t, "1 ahead, 2 behind", AheadBehind{Ahead: 1, Behind: 2}.String())
}
