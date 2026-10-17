package cli

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
)

type printed struct {
	Stdout string
	Stderr string
	Code   int
}

func runIn(t *testing.T, args ...string) printed {
	t.Helper()
	root := NewRoot("1.2.3", []*cobra.Command{probe()})
	var stdout, stderr bytes.Buffer
	root.SetIn(&bytes.Buffer{})
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(args)
	code := Execute(t.Context(), root)
	return printed{Stdout: stdout.String(), Stderr: stderr.String(), Code: code}
}
