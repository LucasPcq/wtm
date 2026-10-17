package shared

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/surface/tui/components"
)

// Load is RunLoading for a read. A listing an interrupt cut short is no answer
// even when the reads under it swallowed their errors — a worktree whose
// status never came back would be shown clean.
func Load(cmd *cobra.Command, params components.LoadingParams) error {
	if err := components.RunLoading(cmd.Context(), params); err != nil {
		return err
	}
	return cmd.Context().Err()
}
