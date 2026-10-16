package envcopy_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules/envcopy"
)

func TestTheParentStrategyFallsBackToMainOnlyWithoutTheSourcesWorktree(t *testing.T) {
	falls := func(params envcopy.ParentFallbackParams) bool { return envcopy.ParentFallsBackToMain(params) }

	assert.True(t, falls(envcopy.ParentFallbackParams{Strategy: domain.EnvStrategyParent, HasCopyFiles: true}))
	assert.False(t, falls(envcopy.ParentFallbackParams{Strategy: domain.EnvStrategyParent, HasCopyFiles: true, SourceHasWorktree: true}))
	assert.False(t, falls(envcopy.ParentFallbackParams{Strategy: domain.EnvStrategyParent}))
	assert.False(t, falls(envcopy.ParentFallbackParams{Strategy: domain.EnvStrategyMain, HasCopyFiles: true}))
	assert.False(t, falls(envcopy.ParentFallbackParams{Strategy: domain.EnvStrategyExample, HasCopyFiles: true}))
}
