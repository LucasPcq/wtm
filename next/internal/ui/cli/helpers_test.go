package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/ui/output"
	"github.com/spf13/cobra"
)

type printed struct {
	Stdout string
	Stderr string
	Code   int
}

func runIn(t *testing.T, args ...string) printed {
	t.Helper()
	streams, _, stdout, stderr := output.Test()
	root := NewRoot("1.2.3", streams, []*cobra.Command{probe()})
	root.SetArgs(args)
	code := Execute(t.Context(), root)
	return printed{Stdout: stdout.String(), Stderr: stderr.String(), Code: code}
}
