package cli

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/cobra"
)

func runShow(t *testing.T, w world, words ...string) formtest.Printed {
	t.Helper()
	dir := t.TempDir()
	open := w.opener(t, dir)
	got := formtest.Run(t.Context(), func(streams *output.Streams) *cobra.Command {
		return New(streams, w.env(), open, formtest.CommandLine)
	}, append([]string{"show"}, words...)...)
	got.Stderr = strings.ReplaceAll(got.Stderr, dir, "<state>")
	return got
}
