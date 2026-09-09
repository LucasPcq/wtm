package run

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/run/runctx"
	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	listflow "github.com/LucasPcq/wtm/internal/flow/run/list"
	"github.com/LucasPcq/wtm/internal/output"
)

// newListCmd creates the wtm run list subcommand.
func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdList,
		Short: "List jobs and profiles declared in run.toml",
		Long:  "Show the jobs and profiles configured for the project.\nIn a TTY, offers an interactive picker with start/stop/logs actions.",
		RunE:  runList,
	}
	shared.AddYesFlag(cmd, "Skip the interactive picker; print the table instead")
	shared.AddOutputFlag(cmd)
	return cmd
}

func runList(cmd *cobra.Command, _ []string) error {
	ctx, err := runctx.Open(runctx.OpenParams{Cmd: cmd})
	if err != nil {
		return err
	}

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	if format == domain.OutputJSON {
		return output.WriteRunConfigJSON(cmd.OutOrStdout(), ctx.Run)
	}

	if !ctx.Interactive || (len(ctx.Run.Jobs) == 0 && len(ctx.Run.Profiles) == 0) {
		output.Frame(cmd.OutOrStdout(), func(w io.Writer) {
			fmt.Fprint(w, output.FormatRunConfig(ctx.Run))
		})
		return nil
	}

	selection, err := listflow.Run(listflow.Params{
		Context:   ctx.FlowContext(),
		Request:   listflow.Request{Config: ctx.Run},
		Prompter:  ctx.Prompter(true),
		Presenter: shared.NewPresenter(cmd, format),
	})
	if err != nil {
		return err
	}
	if selection.Aborted {
		return nil
	}
	return execListAction(cmd, selection, ctx.Dir)
}

// execListAction runs the picked action in this process, on the worktree the
// command was launched from: `run list` shows what run.toml declares here, not
// what is running anywhere.
func execListAction(cmd *cobra.Command, selection listflow.Selection, workDir string) error {
	params := dispatchParams{Cmd: cmd, WorkDir: workDir, Format: domain.OutputText}
	if selection.Kind == domain.RunListKindProfile {
		params.Profile = selection.Name
		switch selection.Action {
		case domain.RunListActionUp:
			return params.dispatchUp()
		case domain.RunListActionDown:
			return params.dispatchDown(false)
		}
		return nil
	}

	params.Job = selection.Name
	switch selection.Action {
	case domain.RunListActionStart:
		return params.dispatchStart()
	case domain.RunListActionStop:
		return params.dispatchStop()
	case domain.RunListActionLogs:
		return params.dispatchLogs()
	}
	return nil
}
