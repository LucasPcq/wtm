package shell

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestALineTheShellParsesPassesTheCheckWithoutRunning(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, Syntax(t.Context(), "touch "+dir+"/ran && echo ok"))

	assert.NoFileExists(t, dir+"/ran")
}

func TestALineTheShellRefusesFailsWithTheShellsMessage(t *testing.T) {
	err := Syntax(t.Context(), "if then")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "syntax error")
}
