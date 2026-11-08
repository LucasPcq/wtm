package cli

import (
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/cobra"
)

func runPs(t *testing.T, w world, words ...string) formtest.Printed {
	t.Helper()
	was := now
	now = func() time.Time { return clock }
	t.Cleanup(func() { now = was })
	return formtest.Run(t.Context(), func(streams *output.Streams) *cobra.Command {
		parent := &cobra.Command{Use: "run"}
		parent.AddCommand(New(streams, w.env(), w.open, formtest.CommandLine))
		return parent
	}, append([]string{"run", "ps"}, words...)...)
}
