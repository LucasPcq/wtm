package cli

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTheRootPrintsItsVersion(t *testing.T) {
	root := NewRoot("1.2.3")
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"--version"})

	code := Execute(context.Background(), root)

	assert.Equal(t, exitOK, code)
	assert.Equal(t, "wtm-next version 1.2.3\n", out.String())
}

func TestAnUnknownFlagFails(t *testing.T) {
	root := NewRoot("1.2.3")
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"--nope"})

	assert.Equal(t, exitError, Execute(context.Background(), root))
}
