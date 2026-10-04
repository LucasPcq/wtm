// Package versioncmd wires `wtm version`, the probe a host runs before relying on
// any of wtm's machine contracts.
package versioncmd

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
)

type NewCmdParams struct {
	Version string
}

func NewCmd(params NewCmdParams) *cobra.Command {
	cmd := &cobra.Command{
		Use:         domain.CmdVersion,
		Annotations: map[string]string{domain.AnnotationMachineOutput: domain.AnnotationOn},
		Short:       "Print wtm's version and the versions of its machine contracts",
		Long: "Print the version of this wtm binary, the same line as `wtm --version`.\n" +
			"With --output json it also gives the version of each contract an integration\n" +
			"reads, so a host can tell it is talking to a wtm it understands before relying on\n" +
			"it: `events` is the schema version of `wtm events`. New keys are added as new\n" +
			"contracts appear; a reader ignores the ones it does not know.",
		Example: `  wtm version

  # What a host checks before reading wtm events
  wtm version --output json | jq .events`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			format, _ := cmd.Flags().GetString(domain.FlagOutput)
			if rules.IsHumanFormat(format) {
				output.VersionLine(cmd.OutOrStdout(), params.Version)
				return nil
			}
			return output.WriteVersionJSON(cmd.OutOrStdout(), domain.VersionReport{
				Version: params.Version,
				Events:  domain.EventsSchemaVersion,
			})
		},
	}
	shared.AddOutputFlag(cmd)
	return cmd
}
