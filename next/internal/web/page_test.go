package web

import (
	"io"
	"io/fs"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestARouteOfTheFrontIsServedThePage(t *testing.T) {
	index, err := fs.ReadFile(front, "index.html")
	require.NoError(t, err)

	for name, path := range map[string]string{
		"root":     "/",
		"deep":     "/r/0a1b2c/worktrees",
		"no_token": "/r/0a1b2c",
	} {
		t.Run(name, func(t *testing.T) {
			response := do(t, serving(t, Params{}), req{method: http.MethodGet, path: path, noToken: true})
			body, err := io.ReadAll(response.Body)
			require.NoError(t, err)

			assert.Equal(t, http.StatusOK, response.StatusCode)
			assert.Contains(t, response.Header.Get("Content-Type"), "text/html")
			assert.Equal(t, "no-cache", response.Header.Get("Cache-Control"))
			assert.Equal(t, string(index), string(body))
		})
	}
}

func TestAMissingAssetIsNotFound(t *testing.T) {
	response := do(t, serving(t, Params{}), req{method: http.MethodGet, path: "/assets/gone-0000.js", noToken: true})

	assert.Equal(t, http.StatusNotFound, response.StatusCode)
	assert.Equal(t, CodeNotFound, decoded[envelope](t, response).Error.Code)
}

func TestThePageAnswersGetOnly(t *testing.T) {
	response := do(t, serving(t, Params{}), req{method: http.MethodPost, path: "/r/0a1b2c/worktrees", noToken: true})

	assert.Equal(t, http.StatusMethodNotAllowed, response.StatusCode)
	assert.Equal(t, "GET, HEAD", response.Header.Get("Allow"))
	assert.Equal(t, CodeMethodNotAllowed, decoded[envelope](t, response).Error.Code)
}
