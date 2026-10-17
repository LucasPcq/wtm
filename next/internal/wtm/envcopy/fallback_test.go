package envcopy

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/wtm/project"
	"github.com/stretchr/testify/assert"
)

func TestTheParentStrategyFallsBackToMainOnlyWithoutTheSourcesWorktree(t *testing.T) {
	assert.True(t, ParentFallsBackToMain(ParentFallbackParams{Strategy: project.EnvParent, HasFiles: true}))
	assert.False(t, ParentFallsBackToMain(ParentFallbackParams{Strategy: project.EnvParent, HasFiles: true, SourceHasWorktree: true}))
	assert.False(t, ParentFallsBackToMain(ParentFallbackParams{Strategy: project.EnvParent}))
	assert.False(t, ParentFallsBackToMain(ParentFallbackParams{Strategy: project.EnvMain, HasFiles: true}))
	assert.False(t, ParentFallsBackToMain(ParentFallbackParams{Strategy: project.EnvExample, HasFiles: true}))
}
