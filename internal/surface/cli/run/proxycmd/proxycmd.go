// Package proxycmd implements `wtm run proxy status|install|uninstall` — the
// machine-wide redirection that lets named URLs drop their port.
package proxycmd

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
)

// NewCmd creates the wtm run proxy command group.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdProxy,
		Short: "Inspect and install the redirection that serves named URLs on port 80",
		Long:  "Named job URLs carry the run proxy's port unless port 80 is redirected to it. These commands report that redirection and install or remove it.",
		Example: `  wtm run proxy status
  wtm run proxy install`,
	}

	cmd.AddCommand(newStatusCmd())
	cmd.AddCommand(newInstallCmd())
	cmd.AddCommand(newUninstallCmd())

	return cmd
}
