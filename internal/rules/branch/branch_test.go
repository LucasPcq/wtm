package branch_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules/branch"
)

func TestABranchStandsAgainstOriginByItsCountsOnBothSides(t *testing.T) {
	assert.Equal(t, domain.DivergenceUpToDate, branch.Divergence(0, 0))
	assert.Equal(t, domain.DivergenceAhead, branch.Divergence(2, 0))
	assert.Equal(t, domain.DivergenceBehind, branch.Divergence(0, 5))
	assert.Equal(t, domain.DivergenceDiverged, branch.Divergence(2, 5))
}

func TestEachStandingHasItsLabelAndNoCounterpartNone(t *testing.T) {
	assert.Equal(t, domain.DivergenceLabelUpToDate, branch.DivergenceLabel(domain.DivergenceUpToDate))
	assert.Equal(t, domain.DivergenceLabelBehind, branch.DivergenceLabel(domain.DivergenceBehind))
	assert.Equal(t, domain.DivergenceLabelAhead, branch.DivergenceLabel(domain.DivergenceAhead))
	assert.Equal(t, domain.DivergenceLabelDiverged, branch.DivergenceLabel(domain.DivergenceDiverged))
	assert.Empty(t, branch.DivergenceLabel(domain.DivergenceUnknown))
}

func TestARemoteRefIsNamedAfterOrigin(t *testing.T) {
	assert.True(t, branch.IsRemote("origin/feature"))
	assert.False(t, branch.IsRemote("feature"))
}

func TestOnlyABranchThatAlreadyExistsHasNoInferableParent(t *testing.T) {
	assert.True(t, branch.ParentMustBeExplicit(domain.BranchTargetExisting))
	assert.False(t, branch.ParentMustBeExplicit(domain.BranchTargetNew))
	assert.False(t, branch.ParentMustBeExplicit(domain.BranchTargetCheckedOut))
}
