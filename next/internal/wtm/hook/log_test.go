package hook

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAHookLogIsNamedAfterItsEscapedBranch(t *testing.T) {
	assert.Equal(t, filepath.Join("/state", "hooks", "on_create-feat%2Fx.log"), LogPath(LogPathParams{StateDir: "/state", Branch: "feat/x"}))
}

func TestNoStateDirMeansNoLog(t *testing.T) {
	assert.Empty(t, LogPath(LogPathParams{Branch: "feat/x"}))
}
