package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAFieldIsSpelledAsTheCommandLineTakesIt(t *testing.T) {
	assert.Equal(t, "<branches>", createFlags.Spell("branches"))
	assert.Equal(t, "<branches>[1]", createFlags.Spell("branches[1]"))
	assert.Equal(t, "--from", createFlags.Spell("from"))
	assert.Equal(t, "--env-from", createFlags.Spell("env_from"))
	assert.Equal(t, "--ff", createFlags.Spell("fast_forward"))
	assert.Equal(t, "--if-not-exists", createFlags.Spell("if_not_exists"))
	assert.Equal(t, "--remember[0]", createFlags.Spell("remember[0]"))
}

func TestAPathTheCommandLineDoesNotKnowIsLeftAsItIs(t *testing.T) {
	assert.Equal(t, "target.base", createFlags.Spell("target.base"))
}
