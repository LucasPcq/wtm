package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTheRootPrintsItsVersion(t *testing.T) {
	got := runIn(t, "--version")

	assert.Equal(t, ExitOK, got.Code)
	assert.Equal(t, "wtm-next version 1.2.3\n", got.Stdout)
}

func TestAnUnknownFlagOfTheRootIsAUsageError(t *testing.T) {
	got := runIn(t, "--nope")

	assert.Equal(t, ExitUsage, got.Code)
	assert.Equal(t, "\n  ✗ unknown flag: --nope\n\n", got.Stderr)
}

func TestAnUnknownCommandIsAUsageError(t *testing.T) {
	got := runIn(t, "nope")

	assert.Equal(t, ExitUsage, got.Code)
	assert.Contains(t, got.Stderr, `✗ unknown command "nope" for "wtm-next"`)
}
