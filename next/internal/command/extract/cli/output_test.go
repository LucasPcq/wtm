package cli

import (
	"bytes"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/stretchr/testify/assert"
)

func TestAnOutcomeWithoutItsTargetDrawsNothing(t *testing.T) {
	var out bytes.Buffer

	drawOutcome(&out, extract.Outcome{Result: &extract.Result{}}, "")

	assert.Empty(t, out.String())
}
