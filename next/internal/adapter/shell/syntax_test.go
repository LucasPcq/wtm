package shell

import (
	"strings"
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
	assert.Contains(t, strings.ToLower(err.Error()), "syntax error", "bash and dash spell it with a capital or not")
}
