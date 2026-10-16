package project

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOnlyTheThreeEnvStrategiesAreValid(t *testing.T) {
	for _, strategy := range EnvStrategies() {
		assert.True(t, strategy.Valid(), strategy)
	}
	assert.False(t, EnvStrategy("copy").Valid())
}

func TestAnEmptyEnvStrategyIsNotGivenRatherThanAValue(t *testing.T) {
	assert.False(t, EnvStrategy("").Valid())
}

func TestTheListIsACopyACallerMayChange(t *testing.T) {
	strategies := EnvStrategies()
	strategies[0] = "copy"

	assert.Equal(t, EnvExample, EnvStrategies()[0])
}
