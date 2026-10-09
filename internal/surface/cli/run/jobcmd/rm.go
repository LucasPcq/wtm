package jobcmd

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
	jobflow "github.com/LucasPcq/wtm/internal/flow/run/job"
	"github.com/LucasPcq/wtm/internal/surface/cli/run/runctx"
	"github.com/LucasPcq/wtm/internal/surface/cli/shared"
)

func newRmCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdRm + " [name]",
		Short: "Remove a job from run.toml",
		Long: "Remove a job from <git-common-dir>/wtm/run.toml.\n\n" +
			"Without an argument, prompts to pick from the existing jobs; under --yes the\n" +
			"argument is required.\n" +
			"Fails if anything names the job — a profile, a runner's runs, a job's touches,\n" +
			"an [[env_port]] or an [[env]] link — or if a worktree still holds data in it\n" +
			"(a shared service's namespace, which clean finds by the job's name), unless\n" +
			"--force is given: the references are then stripped, and that data is left\n" +
			"for you to drop by hand.",
		Example: `  # Pick the job
  wtm run job rm

  wtm run job rm worker --yes

  # Also strip the profiles and links that name it
  wtm run job rm postgres --force --yes`,
		Args: cobra.MaximumNArgs(1),
		RunE: runRm,
	}
	cmd.Flags().Bool(domain.FlagForce, false, "Remove it anyway: strip the profiles, runs, touches, [[env_port]] and [[env]] links naming it")
	shared.AddYesFlag(cmd, "Skip the picker; [name] is then required")
	shared.AddOutputFlag(cmd)
	return cmd
}

func runRm(cmd *cobra.Command, args []string) error {
	ctx, err := runctx.Open(runctx.OpenParams{Cmd: cmd})
	if err != nil {
		return err
	}

	force, _ := cmd.Flags().GetBool(domain.FlagForce)
	outcome, err := jobflow.Remove(cmd.Context(), jobflow.RemoveParams{
		Context: ctx.FlowContext(),
		Request: jobflow.RemoveRequest{
			Name:   runctx.FirstArg(args),
			Force:  force,
			Config: ctx.Run,
		},
		Prompter:  ctx.Prompter(cmd.Context(), ctx.Interactive),
		Presenter: presenter{CLIPresenter: ctx.CLI(cmd)},
	})
	if err != nil {
		return err
	}
	if outcome.Aborted {
		return domain.ErrAborted
	}
	return nil
}
