package run

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
	startflow "github.com/LucasPcq/wtm/internal/flow/run/start"
	"github.com/LucasPcq/wtm/internal/surface/cli/run/runctx"
	"github.com/LucasPcq/wtm/internal/surface/cli/shared"
)

// newStartCmd creates the wtm run start subcommand.
func newStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdStart + " [worktree]",
		Short: "Start a single job",
		Long: "Start one job of [worktree] — the current one when omitted, picked interactively when there is a terminal.\n" +
			"The job is named with --job; without it, a fully interactive run offers a picker.\n" +
			"A service attaches: its output opens in the run view, and leaving the view detaches without stopping it.\n" +
			"-d starts it and returns the prompt instead.\n" +
			"A task always runs inline and blocks until it exits, with or without -d.\n" +
			"Like `run up`, it reports what run.toml gets wrong before starting, checks the job's declared ports\n" +
			"once it is up (see --no-probe and run.toml's port_probe_timeout), and asks once what to do about\n" +
			"the jobs other worktrees are running; --exclusive and --parallel answer for one run.",
		Example: `  # Pick the job to start in this worktree
  wtm run start

  wtm run start --job api

  # A task runs inline, to the end
  wtm run start feat/login --job migrate --yes

  wtm run start feat/login --job api -d --yes --output json`,
		Args: cobra.MaximumNArgs(1),
		RunE: runStart,
	}
	shared.AddJobFlag(cmd, "Job to start (required without a terminal or in --output json mode)")
	cmd.Flags().Bool(domain.FlagExclusive, false, "Stop jobs on other worktrees before starting")
	cmd.Flags().Bool(domain.FlagParallel, false, "Start without stopping other worktrees")
	cmd.MarkFlagsMutuallyExclusive(domain.FlagExclusive, domain.FlagParallel)
	cmd.Flags().BoolP(domain.FlagDetach, "d", false, "Start the service and return immediately instead of opening its output")
	cmd.Flags().Bool(domain.FlagNoProbe, false, "Skip the check that each declared port was actually bound")
	cmd.Flags().Bool(domain.FlagForce, false, "Lift the refusal to start a job whose touches reach foreign data (see wtm run --help); other questions are still asked unless --yes")
	shared.AddYesFlag(cmd, "Skip all prompts; --job is then required, and the other worktrees' jobs keep running unless --exclusive")
	shared.AddOutputFlag(cmd)
	return cmd
}

func runStart(cmd *cobra.Command, args []string) error {
	ctx, err := runctx.Open(runctx.OpenParams{Cmd: cmd})
	if err != nil {
		return err
	}
	if err := reportRunConfig(cmd, ctx.Run); err != nil {
		return err
	}

	warnIndexFrozen(cmd)

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	detach, _ := cmd.Flags().GetBool(domain.FlagDetach)
	job, _ := cmd.Flags().GetString(domain.FlagJob)
	force, _ := cmd.Flags().GetBool(domain.FlagForce)
	exclusive, _ := cmd.Flags().GetBool(domain.FlagExclusive)
	parallel, _ := cmd.Flags().GetBool(domain.FlagParallel)
	noProbe, _ := cmd.Flags().GetBool(domain.FlagNoProbe)

	outcome, err := startflow.Run(cmd.Context(), startflow.Params{
		Context: ctx.FlowContext(),
		Request: startflow.Request{
			Worktree:  runctx.FirstArg(args),
			Cwd:       ctx.Dir,
			Job:       job,
			Exclusive: exclusive,
			Parallel:  parallel,
			NoProbe:   noProbe,
			Force:     force,
			Config:    ctx.Run,
		},
		Prompter:  ctx.Prompter(cmd.Context(), ctx.Interactive),
		Presenter: startPresenter{CLIPresenter: shared.NewPresenter(cmd, format), detach: detach},
	})
	if err != nil {
		return err
	}
	if outcome.Aborted {
		return domain.ErrAborted
	}
	return nil
}
