package output

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAFieldIsSpelledAsTheCommandLineTakesIt(t *testing.T) {
	assert.Equal(t, "<branches>", Flag("branches"))
	assert.Equal(t, "<branches>[1]", Flag("branches[1]"))
	assert.Equal(t, "--from", Flag("from"))
	assert.Equal(t, "--env-from", Flag("env_from"))
	assert.Equal(t, "--ff", Flag("fast_forward"))
	assert.Equal(t, "--if-not-exists", Flag("if_not_exists"))
	assert.Equal(t, "--remember[0]", Flag("remember[0]"))
}
