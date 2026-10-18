package output

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestColumnsAlignCellsByTheColumnsTheyTakeNotTheirBytes(t *testing.T) {
	coloured := "\x1b[32m● active\x1b[0m"

	got := rendered(func(w *bytes.Buffer) {
		Columns(w, [][]string{
			{"main", coloured, "", "✓ clean"},
			{"feat/a", "", "base ↑2", "! dirty"},
			{"feat/b", "! locked", "", ""},
		})
	})

	assert.Equal(t, ""+
		"  main    "+coloured+"           ✓ clean\n"+
		"  feat/a            base ↑2  ! dirty\n"+
		"  feat/b  ! locked\n", got)
}
