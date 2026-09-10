package run

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/run/runctx"
	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	upflow "github.com/LucasPcq/wtm/internal/flow/run/up"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
)

// newUpCmd creates the wtm run up subcommand.
func newUpCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdUp + " [worktree...]",
		Short: "Start a profile's jobs",
		Long: "Start every job in a profile, in declared order, in each [worktree] — the current one when omitted, picked interactively when there is a terminal.\n" +
			"Several worktrees start concurrently and independently: one that aborts leaves the others running,\n" +
			"and the run exits non-zero if any of them did.\n" +
			"Without --profile, uses the default profile (or shows a picker if multiple exist).\n" +
			"Once the jobs are up, each declared port is checked: a port nothing answers on is\n" +
			"reported rather than announced as bound. It never fails the run — see --no-probe\n" +
			"and run.toml's port_probe_timeout.\n" +
			"Tasks block the profile and abort it on failure; services launch in the background.\n" +
			"When another worktree is already running jobs, wtm asks once what to do about it and can\n" +
			"remember the answer as run.toml's `concurrency`; --exclusive and --parallel override it\n" +
			"for one run. --exclusive is refused on several worktrees, since it stops all but one.\n" +
			"The run view opens on the jobs as they start; leaving it detaches without stopping them, and -d skips it.",
		Args: cobra.ArbitraryArgs,
		RunE: runUp,
	}

	shared.AddProfilesFlag(cmd, "Profiles to start; repeatable. A job several of them name starts once. Defaults to the default profile, or a picker when several exist")
	cmd.Flags().Bool(domain.FlagExclusive, false, "Stop jobs on other worktrees before starting (one worktree only)")
	cmd.Flags().Bool(domain.FlagParallel, false, "Start without stopping other worktrees")
	cmd.MarkFlagsMutuallyExclusive(domain.FlagExclusive, domain.FlagParallel)
	cmd.Flags().BoolP(domain.FlagDetach, "d", false, "Start the jobs and return immediately instead of opening their output")
	cmd.Flags().Bool(domain.FlagNoProbe, false, "Skip the check that each declared port was actually bound")
	shared.AddYesFlag(cmd, "Skip all prompts; leaves the other worktrees' jobs running unless --exclusive")
	shared.AddOutputFlag(cmd)

	return cmd
}

func runUp(cmd *cobra.Command, args []string) error {
	ctx, err := runctx.Open(runctx.OpenParams{Cmd: cmd})
	if err != nil {
		return err
	}
	if err := reportRunConfig(cmd, ctx.Run); err != nil {
		return err
	}

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	detach, _ := cmd.Flags().GetBool(domain.FlagDetach)
	exclusive, _ := cmd.Flags().GetBool(domain.FlagExclusive)
	parallel, _ := cmd.Flags().GetBool(domain.FlagParallel)
	noProbe, _ := cmd.Flags().GetBool(domain.FlagNoProbe)
	profiles, _ := cmd.Flags().GetStringSlice(domain.FlagProfile)

	outcome, err := upflow.Run(upflow.Params{
		Context: ctx.FlowContext(),
		Request: upflow.Request{
			Worktrees: args,
			Cwd:       ctx.Dir,
			Profiles:  profiles,
			Exclusive: exclusive,
			Parallel:  parallel,
			NoProbe:   noProbe,
			Config:    ctx.Run,
		},
		// The run wizard may be reached through the shell wrapper, which
		// consumes stdout.
		Prompter:  ctx.Prompter(ctx.Interactive),
		Presenter: upPresenter{CLIPresenter: shared.NewPresenter(cmd, format), detach: detach},
	})
	if err != nil {
		return err
	}
	return concluded(outcome)
}

// reportRunConfig prints what the config got wrong before anything is started:
// warnings are advice, errors refuse the run.
func reportRunConfig(cmd *cobra.Command, cfg domain.RunConfig) error {
	warnings, errs := rules.ValidateRun(cfg)
	if len(warnings) == 0 && len(errs) == 0 {
		return nil
	}
	output.Frame(cmd.ErrOrStderr(), func(w io.Writer) {
		for _, warning := range warnings {
			output.Warning(w, warning)
		}
		for _, e := range errs {
			output.Error(w, e)
		}
	})
	if len(errs) == 0 {
		return nil
	}
	// The block above IS the report; ErrAborted is how a command says so without
	// Execute printing a second, emptier line under it. The cause rides along for
	// the run whose report went to io.Discard.
	return fmt.Errorf("%w: %s", domain.ErrAborted, errs[0])
}
