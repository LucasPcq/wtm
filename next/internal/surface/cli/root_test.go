package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTheRootPrintsItsVersion(t *testing.T) {
	got := runIn(t, newWorld(seed{}), "--version")

	assert.Equal(t, exitOK, got.Code)
	assert.Equal(t, "wtm-next version 1.2.3\n", got.Stdout)
}

func TestAnUnknownFlagOfTheRootIsAUsageError(t *testing.T) {
	got := runIn(t, newWorld(seed{}), "--nope")

	assert.Equal(t, exitUsage, got.Code)
	assert.Equal(t, "\n  ✗ unknown flag: --nope\n\n", got.Stderr)
}

func TestAnUnknownCommandIsAUsageError(t *testing.T) {
	got := runIn(t, newWorld(seed{}), "nope")

	assert.Equal(t, exitUsage, got.Code)
	assert.Contains(t, got.Stderr, `✗ unknown command "nope" for "wtm-next"`)
}
