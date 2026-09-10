package jobcmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/run/runctx"
	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	jobflow "github.com/LucasPcq/wtm/internal/flow/run/job"
	"github.com/LucasPcq/wtm/internal/output"
)

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdList,
		Short: "List jobs from run.toml",
		Long: "List jobs declared in <git-common-dir>/wtm/run.toml.\n\n" +
			"In a TTY, opens an interactive picker. Selecting a job offers Edit or Remove.\n" +
			"Use --output json, --yes (or pipe stdout) for a non-interactive listing.",
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
		JSON: func(w io.Writer) error { return output.WriteJobsJSON(w, ctx.Run.Jobs) },
		Table: func(w io.Writer) {
			fmt.Fprint(w, output.FormatRunConfig(output.FormatRunConfigParams{Config: domain.RunConfig{Jobs: ctx.Run.Jobs}, Empty: domain.RunJobsEmpty}))
		},
	})
	if answered || err != nil {
		return err
	}

	// Backing out of a listing is not a failure: nothing was asked for.
	_, err = jobflow.List(jobflow.ListParams{
		Context:   ctx.FlowContext(),
		Request:   jobflow.ListRequest{Config: ctx.Run},
		Prompter:  ctx.Prompter(ctx.Interactive),
		Presenter: presenter{CLIPresenter: ctx.CLI(cmd)},
	})
	return err
}
