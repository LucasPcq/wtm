// Package configcmd exposes the `wtm config` command group: show and edit.
package configcmd

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
)

// NewCmd creates the wtm config command tree.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect or edit the project wtm config",
		Long:  "View the resolved config or open the project config.toml in $EDITOR.\nThe file lives under <git-common-dir>/wtm/config.toml and is never committed.",
		Example: `  wtm config show
  wtm config edit`,
		GroupID: domain.CmdGroupSetup,
	}
	cmd.AddCommand(newShowCmd())
	cmd.AddCommand(newEditCmd())
	return cmd
}
