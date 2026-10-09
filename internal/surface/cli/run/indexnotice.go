package run

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/process"
)

// warnIndexFrozen says, before anything is started, that nothing about it will be
// recorded. A read-only index used to be entirely silent, and it is the moment a
// job goes up that the loss is actually taken: the entry the next daemon would
// have read is the one never written. On stderr, so the warning never reaches a
// caller reading the run's output.
func warnIndexFrozen(cmd *cobra.Command) {
	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	if !rules.IsHumanFormat(format) || !process.IndexFrozen() {
		return
	}
	output.Frame(cmd.ErrOrStderr(), func(w io.Writer) {
		output.Callout(w, domain.DaemonIndexFrozenTitle, rules.IndexFrozenLines(process.StatePath()))
	})
}
