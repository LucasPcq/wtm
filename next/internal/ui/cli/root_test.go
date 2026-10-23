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
	assert.Equal(t, "\n  ✗ the request is invalid: unknown flag: --nope\n  → wtm-next --help\n\n", got.Stderr)
}

func TestAnUnknownCommandIsAUsageError(t *testing.T) {
	got := runIn(t, "nope")

	assert.Equal(t, ExitUsage, got.Code)
	assert.Contains(t, got.Stderr, `✗ the request is invalid: unknown command "nope" for "wtm-next"`)
	assert.Contains(t, got.Stderr, "→ wtm-next --help")
}

func TestAUsageErrorOfTheRootIsInTheEnvelopeWhenJSONWasAsked(t *testing.T) {
	got := runIn(t, "nope", "--output", "json")

	assert.Equal(t, ExitUsage, got.Code)
	assert.Empty(t, got.Stderr)
	assert.Contains(t, got.Stdout, `"message": "the request is invalid: unknown command \"nope\" for \"wtm-next\""`)
}

func TestARefusalOffersItsWayOutUnderQuietToo(t *testing.T) {
	loud := runIn(t, "refuse")
	quiet := runIn(t, "refuse", "-q")

	assert.Equal(t, ExitConfigNotFound, quiet.Code)
	assert.Equal(t, "\n  ✗ no wtm config found\n  → wtm init\n\n", loud.Stderr)
	assert.Equal(t, loud.Stderr, quiet.Stderr)
}

// Through the event stream, a working directory removed under the command line widened it to every repository.
func TestACommandLineThatCannotReadItsWorkingDirectoryIsRefused(t *testing.T) {
	got := runIn(t, "lost")

	assert.Equal(t, ExitPrecondition, got.Code)
	assert.Equal(t, "\n  ✗ the current directory cannot be read (removed?): cd into an existing directory\n\n", got.Stderr)
}
