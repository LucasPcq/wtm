package run

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/run/runctx"
	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	stopflow "github.com/LucasPcq/wtm/internal/flow/run/stop"
)

// newStopCmd creates the wtm run stop subcommand.
func newStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdStop + " [worktree...]",
		Short: "Stop one job, in one or more worktrees",
		Long:  "Stop one running job in each [worktree] — the current one when omitted, picked interactively when there is a terminal.\nThe job is named with --job; without it, a fully interactive run offers a picker.",
		Example: `  wtm run stop --job api

  wtm run stop feat/login fix/typo --job web --yes`,
		Args: cobra.ArbitraryArgs,
		RunE: runStop,
	}
	shared.AddJobFlag(cmd, "Job to stop (required without a terminal or in --output json mode)")
	shared.AddYesFlag(cmd, "Skip all prompts; --job is then required")
	shared.AddOutputFlag(cmd)
	return cmd
}

func runStop(cmd *cobra.Command, args []string) error {
	ctx, err := runctx.Open(runctx.OpenParams{Cmd: cmd, TolerateRunConfig: true})
	if err != nil {
		return err
	}
	warnRunConfig(cmd, ctx)

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	job, _ := cmd.Flags().GetString(domain.FlagJob)

	outcome, err := stopflow.Run(cmd.Context(), stopflow.Params{
		Context: ctx.FlowContext(),
		Request: stopflow.Request{
			Worktrees: args,
			Cwd:       ctx.Dir,
			Job:       job,
			Config:    ctx.Run,
			ByName:    ctx.RunErr != nil,
		},
		Prompter:  ctx.Prompter(ctx.Interactive),
		Presenter: stopPresenter{CLIPresenter: shared.NewPresenter(cmd, format)},
	})
	if err != nil {
		return err
	}
	if outcome.Aborted {
		return shared.EndAborted(cmd)
	}
	return nil
}
