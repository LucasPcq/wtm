//go:build noui

package web

import (
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestABuildWithoutTheFrontSaysSo(t *testing.T) {
	response := do(t, serving(t, Params{}), req{method: http.MethodGet, path: "/r/0a1b2c/worktrees", noToken: true})
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Contains(t, string(body), "This wtm has no web interface")
}
