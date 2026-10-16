package runenv

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/wtm/run"
	"github.com/stretchr/testify/assert"
)

func TestAVerbatimWorktreeIsLeftAlone(t *testing.T) {
	assert.Equal(t, Verbatim{}, CaseOf(run.Verbatim))
}

func TestAnIsolatedWorktreeIsLinked(t *testing.T) {
	assert.Equal(t, Linked{}, CaseOf(run.Isolated))
}

func TestAnUnrecordedIsolationReadsAsIsolated(t *testing.T) {
	assert.Equal(t, Linked{}, CaseOf(""))
}
