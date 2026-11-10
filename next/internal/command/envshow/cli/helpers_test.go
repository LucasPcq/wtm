package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
	"github.com/spf13/cobra"
)

func runShow(t *testing.T, words ...string) formtest.Printed {
	t.Helper()
	return formtest.Run(t.Context(), func(streams *output.Streams) *cobra.Command {
		return New(streams, cliEnv(), open, formtest.CommandLine)
	}, append([]string{"show"}, words...)...)
}
