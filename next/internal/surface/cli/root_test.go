package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTheRootPrintsItsVersion(t *testing.T) {
	root := NewRoot("1.2.3")
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"--version"})

	code := Execute(t.Context(), root)

	assert.Equal(t, exitOK, code)
	assert.Equal(t, "wtm-next version 1.2.3\n", out.String())
}

func TestAnUnknownFlagFails(t *testing.T) {
	root := NewRoot("1.2.3")
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"--nope"})

	assert.Equal(t, exitError, Execute(t.Context(), root))
}
