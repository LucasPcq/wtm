package field

import (
	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/spf13/cobra"
)

func newCmd() *cobra.Command { return &cobra.Command{} }

func run(interactive bool) bool {
	return shared.Prompter(shared.PrompterParams{Interactive: interactive})
}
