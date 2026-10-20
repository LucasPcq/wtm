package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/tree"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/cobra"
)

type printed struct {
	Stdout string
	Stderr string
	Code   int
}

func runTree(t *testing.T, w world, words ...string) printed {
	t.Helper()
	streams, _, stdout, stderr := output.Test()
	root := uicli.NewRoot("1.2.3", streams, []*cobra.Command{New(streams, w.env(), w.open, formtest.CommandLine)})
	code := uicli.Execute(t.Context(), root, append([]string{tree.Name}, words...))
	return printed{Stdout: stdout.String(), Stderr: stderr.String(), Code: code}
}
