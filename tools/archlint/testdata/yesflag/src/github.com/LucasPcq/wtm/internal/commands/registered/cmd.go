package registered

import (
	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/spf13/cobra"
)

func newCmd() *cobra.Command {
	cmd := &cobra.Command{}
	shared.AddYesFlag(cmd)
	return cmd
}

func run() bool { return shared.Interactive() }
