package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAFieldIsSpelledAsTheCommandLineTakesIt(t *testing.T) {
	assert.Equal(t, "<source>", extractFlags.Spell("source"))
	assert.Equal(t, "--files", extractFlags.Spell("files"))
	assert.Equal(t, "--ff", extractFlags.Spell("fast_forward"))
	assert.Equal(t, "--on-conflict", extractFlags.Spell("on_conflict"))
	assert.Equal(t, "--remember[0]", extractFlags.Spell("remember[0]"))
}
