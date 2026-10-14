package render_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LucasPcq/wtm/internal/surface/cli/render"
)

func TestAViewIsItsFieldsUnderTheQuerysName(t *testing.T) {
	var out bytes.Buffer

	render.View(&out, "inventory", json.RawMessage(`{"worktrees":["feat/a"]}`))

	assert.Equal(t, lines("  inventory", "  worktrees  feat/a"), plain(out.String()))
}
