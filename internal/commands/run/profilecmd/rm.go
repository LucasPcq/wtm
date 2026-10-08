package profilecmd

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/run/runctx"
	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	profileflow "github.com/LucasPcq/wtm/internal/flow/run/profile"
)

func newRmCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdRm + " [name]",
		Short: "Remove a profile from run.toml",
		Long: "Remove a profile from <git-common-dir>/wtm/run.toml.\n\n" +
			"Without an argument, prompts to pick from the existing profiles; under --yes\n" +
			"the argument is required.\n" +
			"Jobs referenced by the profile are left untouched.",
		Example: `  # Pick the profile
  wtm run profile rm

  wtm run profile rm backend --yes`,
		Args: cobra.MaximumNArgs(1),
		RunE: runRm,
	}
	shared.AddYesFlag(cmd, "Skip the picker; [name] is then required")
	shared.AddOutputFlag(cmd)
	return cmd
}

func runRm(cmd *cobra.Command, args []string) error {
	ctx, err := runctx.Open(runctx.OpenParams{Cmd: cmd})
	if err != nil {
		return err
	}

	outcome, err := profileflow.Remove(cmd.Context(), profileflow.RemoveParams{
		Context:   ctx.FlowContext(),
		Request:   profileflow.RemoveRequest{Name: runctx.FirstArg(args), Config: ctx.Run},
		Prompter:  ctx.Prompter(cmd.Context(), ctx.Interactive),
		Presenter: presenter{CLIPresenter: ctx.CLI(cmd)},
	})
	if err != nil {
		return err
	}
	if outcome.Aborted {
		return shared.EndAborted(cmd)
	}
	return nil
}
