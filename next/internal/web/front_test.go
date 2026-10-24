//go:build !noui

package web

import (
	"io"
	"io/fs"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheFrontIsEmbedded(t *testing.T) {
	index, err := fs.ReadFile(front, "index.html")
	require.NoError(t, err)

	assert.Contains(t, string(index), `<div id="root"></div>`)
}

func TestAnAssetIsServedCachedForever(t *testing.T) {
	scripts, err := fs.Glob(front, "assets/*.js")
	require.NoError(t, err)
	require.NotEmpty(t, scripts)
	want, err := fs.ReadFile(front, scripts[0])
	require.NoError(t, err)

	response := do(t, serving(t, Params{}), req{method: http.MethodGet, path: "/" + scripts[0], noToken: true})
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Contains(t, response.Header.Get("Content-Type"), "javascript")
	assert.Contains(t, response.Header.Get("Cache-Control"), "immutable")
	assert.Equal(t, want, body)
}

func TestTheFrontCarriesNoSourceMap(t *testing.T) {
	maps, err := fs.Glob(front, "assets/*.map")
	require.NoError(t, err)

	assert.Empty(t, maps)
}
