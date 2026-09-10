package run

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/run/runctx"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
)

// newExportCmd creates the wtm run export subcommand.
func newExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         domain.CmdExport,
		Annotations: map[string]string{domain.AnnotationMachineOutput: domain.AnnotationOn},
		Short:       "Export run.toml as JSON on stdout",
		Long:        "Emit the current run config as JSON. Pipe to a file and use with wtm run import to share configurations.",
		RunE:        runExport,
	}
	cmd.Flags().String(domain.FlagProfile, "", "Export only this profile and its jobs")
	return cmd
}

func runExport(cmd *cobra.Command, _ []string) error {
	ctx, err := runctx.Open(runctx.OpenParams{Cmd: cmd})
	if err != nil {
		return err
	}

	profile, _ := cmd.Flags().GetString(domain.FlagProfile)
	if profile != "" {
		ctx.Run, err = rules.FilterToProfile(ctx.Run, profile)
		if err != nil {
			return err
		}
	}

	return output.WriteRunConfigJSON(cmd.OutOrStdout(), ctx.Run)
}
