package output

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHelpIsIndentedBetweenBlankLines(t *testing.T) {
	assert.Equal(t, "\n  Usage:\n\n  Flags:\n\n", rendered(func(w *bytes.Buffer) { Help(w, "Usage:\n\nFlags:\n") }))
}
