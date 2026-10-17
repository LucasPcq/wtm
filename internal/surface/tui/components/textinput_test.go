package components

import (
	"strings"
	"testing"
)

// An edit form opens on the value run.toml holds; a namespace create command
// runs past 256 characters, and the field cut it — enter saved half a command.
func TestTextInputKeepsALongDefaultWhole(t *testing.T) {
	long := strings.Repeat("x", 600)

	if got := NewTextInput(NewTextInputParams{Default: long}).Value(); got != long {
		t.Errorf("default truncated to %d characters", len(got))
	}
}
