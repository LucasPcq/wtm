package branch

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/stretchr/testify/assert"
)

func TestABranchStandsAgainstOriginByItsCountsOnBothSides(t *testing.T) {
	assert.Equal(t, wtm.UpToDate, Divergence(wtm.AheadBehind{}))
	assert.Equal(t, wtm.Behind, Divergence(wtm.AheadBehind{Behind: 2}))
	assert.Equal(t, wtm.Ahead, Divergence(wtm.AheadBehind{Ahead: 1}))
	assert.Equal(t, wtm.Diverged, Divergence(wtm.AheadBehind{Ahead: 1, Behind: 2}))
}

func TestARemoteBranchIsNamedAfterOrigin(t *testing.T) {
	assert.True(t, IsRemote("origin/feat/x"))
	assert.False(t, IsRemote("feat/origin"))
}
