package wtm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOnlyTheThreeEnvStrategiesAreValid(t *testing.T) {
	for _, strategy := range EnvStrategies {
		assert.True(t, strategy.Valid(), strategy)
	}
	assert.False(t, EnvStrategy("copy").Valid())
	assert.False(t, EnvStrategy("").Valid())
}
