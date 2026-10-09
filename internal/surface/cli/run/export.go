package run

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/surface/cli/render"
	"github.com/LucasPcq/wtm/internal/surface/cli/run/runctx"
	"github.com/LucasPcq/wtm/internal/surface/cli/shared"
)

// newExportCmd creates the wtm run export subcommand.
func newExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         domain.CmdExport,
		Annotations: map[string]string{domain.AnnotationMachineOutput: domain.AnnotationOn},
		Short:       "Export run.toml as JSON on stdout",
		Long:        "Emit the current run config as JSON on stdout, whatever --output says: like run url, this is machine output and is never framed. Pipe to a file and use with wtm run import to share configurations.",
		Example: `  wtm run export > run.json

  # One profile and its jobs
  wtm run export --profile backend > backend.json

  # Copy the layout into another clone
  wtm run export | (cd ../other-clone && wtm run import - --yes)`,
		RunE: runExport,
	}
	shared.AddProfileFlag(cmd, "Export only this profile and its jobs")
	shared.AddOutputFlag(cmd)
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

	return render.WriteRunConfigJSON(cmd.OutOrStdout(), ctx.Run)
}
