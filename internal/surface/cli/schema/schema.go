package schema

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
)

// NewCmd creates the wtm schema command group — exposes the JSON
// Schema files bundled with this binary so editors (Taplo, etc.) can offer
// autocomplete and validation on TOML config files.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schema",
		Short: "Inspect or extract bundled JSON Schemas",
		Long:  "JSON Schemas describe the structure of wtm's TOML config files.\nUse `wtm schema dump` to write them to <git-common-dir>/wtm/schemas/ so editors can pick them up via the `#:schema` directive.",
		Example: `  wtm schema dump
  wtm schema dump --global`,
		GroupID: domain.CmdGroupSetup,
	}
	cmd.AddCommand(newDumpCmd())
	return cmd
}
