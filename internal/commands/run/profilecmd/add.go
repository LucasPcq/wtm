package profilecmd

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/run/runctx"
	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	profileflow "github.com/LucasPcq/wtm/internal/flow/run/profile"
)

func newAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdAdd + " [name]",
		Short: "Add a profile to run.toml",
		Long: "Append a profile to <git-common-dir>/wtm/run.toml.\n\n" +
			"Every flag pre-fills the corresponding question, so the form opens on what was\n" +
			"already given. --yes skips the questions altogether: [name] and --jobs are then\n" +
			"required, and the profile is not the default unless --default says so.",
		Example: `  # Answer the form
  wtm run profile add

  wtm run profile add backend --jobs postgres,migrate,api --yes

  # What run up starts without --profile
  wtm run profile add full --jobs postgres,api,web --default --yes`,
		Args: cobra.MaximumNArgs(1),
		RunE: runAdd,
	}
	cmd.Flags().StringSlice(domain.FlagJobs, nil, "Comma-separated existing job names, in start order")
	cmd.Flags().Bool(domain.FlagDefault, false, "Mark this profile as the default")
	shared.AddYesFlag(cmd, "Skip all prompts; [name] and --jobs are then required")
	shared.AddOutputFlag(cmd)
	return cmd
}

func runAdd(cmd *cobra.Command, args []string) error {
	// The creation path, like `run job add`: a run.toml declaring nothing yet is
	// what this command is for.
	ctx, err := runctx.Open(runctx.OpenParams{Cmd: cmd, SkipGuard: true})
	if err != nil {
		return err
	}

	jobs, _ := cmd.Flags().GetStringSlice(domain.FlagJobs)
	isDefault, _ := cmd.Flags().GetBool(domain.FlagDefault)

	outcome, err := profileflow.Add(profileflow.AddParams{
		Context: ctx.FlowContext(),
		Request: profileflow.AddRequest{
			Initial: domain.ProfileConfig{Name: runctx.FirstArg(args), Jobs: jobs, Default: isDefault},
			Config:  ctx.Run,
		},
		Prompter:  ctx.Prompter(ctx.Interactive),
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
