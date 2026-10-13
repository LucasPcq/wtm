package runenv_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules/runenv"
)

func TestAVerbatimWorktreeIsLeftAloneWhateverElseItIs(t *testing.T) {
	assert.Equal(t, runenv.Verbatim{}, runenv.CaseOf(runenv.CaseParams{Isolation: domain.IsolationVerbatim}))
	assert.Equal(t, runenv.Verbatim{}, runenv.CaseOf(runenv.CaseParams{Isolation: domain.IsolationVerbatim, IsMain: true}))
}

func TestTheMainCheckoutHasItsOwnEnvironment(t *testing.T) {
	assert.Equal(t, runenv.Main{}, runenv.CaseOf(runenv.CaseParams{Isolation: domain.IsolationIsolated, IsMain: true}))
}

func TestAnUnrecordedIsolationReadsAsIsolated(t *testing.T) {
	assert.Equal(t, runenv.Linked{}, runenv.CaseOf(runenv.CaseParams{}))
	assert.Equal(t, runenv.Main{}, runenv.CaseOf(runenv.CaseParams{IsMain: true}))
}
