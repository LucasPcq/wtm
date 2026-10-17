package profilecmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
	profileflow "github.com/LucasPcq/wtm/internal/flow/run/profile"
	"github.com/LucasPcq/wtm/internal/surface/cli/render"
	"github.com/LucasPcq/wtm/internal/surface/cli/run/runctx"
	"github.com/LucasPcq/wtm/internal/surface/cli/shared"
)

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdList,
		Short: "List profiles from run.toml",
		Long: "List profiles declared in <git-common-dir>/wtm/run.toml.\n\n" +
			"In a TTY, opens an interactive picker. Selecting a profile offers Edit or Remove.\n" +
			"Use --output json, --yes (or pipe stdout) for a non-interactive listing.",
		Example: `  wtm run profile list

  wtm run profile list --output json`,
		RunE: runList,
	}
	shared.AddYesFlag(cmd, "Skip the picker; print the table instead")
	shared.AddOutputFlag(cmd)
	return cmd
}

func runList(cmd *cobra.Command, _ []string) error {
	ctx, err := runctx.Open(runctx.OpenParams{Cmd: cmd})
	if err != nil {
		return err
	}

	answered, err := ctx.Listing(runctx.ListingParams{
		Cmd:  cmd,
		JSON: func(w io.Writer) error { return render.WriteProfilesJSON(w, ctx.Run.Profiles) },
		Table: func(w io.Writer) {
			fmt.Fprint(w, render.FormatRunConfig(render.FormatRunConfigParams{Config: domain.RunConfig{Profiles: ctx.Run.Profiles}, Empty: domain.RunProfilesEmpty}))
		},
	})
	if answered || err != nil {
		return err
	}

	// Backing out of a listing is not a failure: nothing was asked for.
	_, err = profileflow.List(cmd.Context(), profileflow.ListParams{
		Context:   ctx.FlowContext(),
		Request:   profileflow.ListRequest{Config: ctx.Run},
		Prompter:  ctx.Prompter(cmd.Context(), ctx.Interactive),
		Presenter: presenter{CLIPresenter: ctx.CLI(cmd)},
	})
	return err
}
