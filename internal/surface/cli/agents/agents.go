package agents

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
)

// NewCmd creates the wtm agents command group.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "agents",
		Short:   "Manage LLM agent integrations for wtm",
		GroupID: domain.CmdGroupSetup,
	}
	cmd.AddCommand(newInstallCmd())
	return cmd
}
