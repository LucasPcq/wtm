package profilecmd

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/run/runctx"
	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	profileflow "github.com/LucasPcq/wtm/internal/flow/run/profile"
	"github.com/LucasPcq/wtm/internal/rules"
)

func newEditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdEdit + " [name]",
		Short: "Edit an existing profile",
		Long: "Edit a profile declared in <git-common-dir>/wtm/run.toml.\n\n" +
			"Pass --name, --jobs or --default to change those fields and nothing else: a\n" +
			"flag left out keeps the field as it is. --jobs replaces the whole list — its\n" +
			"order is the start order, so it is given in full — and --default=false takes\n" +
			"the default away without handing it to another profile.\n\n" +
			"With no such flag, the form opens pre-filled with the current values, and\n" +
			"without an argument it prompts to pick from the existing profiles.",
		Example: `  # The form, pre-filled
  wtm run profile edit backend

  # --jobs replaces the list, in start order
  wtm run profile edit backend --jobs postgres,api --yes

  wtm run profile edit backend --default --yes`,
		Args: cobra.MaximumNArgs(1),
		RunE: runEdit,
	}
	shared.AddSingleFlag(cmd, domain.FlagName, "Rename the profile")
	cmd.Flags().StringSlice(domain.FlagJobs, nil, "Comma-separated existing job names, in start order (replaces the list)")
	cmd.Flags().Bool(domain.FlagDefault, false, "Mark this profile as the default (--default=false takes it away)")
	shared.AddYesFlag(cmd, "Skip all prompts; a field flag is then required")
	shared.AddOutputFlag(cmd)
	return cmd
}

// profilePatchFromFlags reads the edit flags as a patch: only what the user
// actually passed, so an absent flag can be told from an explicit value.
func profilePatchFromFlags(cmd *cobra.Command) rules.ProfilePatch {
	var patch rules.ProfilePatch
	if cmd.Flags().Changed(domain.FlagName) {
		name, _ := cmd.Flags().GetString(domain.FlagName)
		patch.Name = &name
	}
	if cmd.Flags().Changed(domain.FlagJobs) {
		jobs, _ := cmd.Flags().GetStringSlice(domain.FlagJobs)
		patch.Jobs = jobs
	}
	if cmd.Flags().Changed(domain.FlagDefault) {
		isDefault, _ := cmd.Flags().GetBool(domain.FlagDefault)
		patch.Default = &isDefault
	}
	return patch
}

func runEdit(cmd *cobra.Command, args []string) error {
	ctx, err := runctx.Open(runctx.OpenParams{Cmd: cmd})
	if err != nil {
		return err
	}

	outcome, err := profileflow.Edit(cmd.Context(), profileflow.EditParams{
		Context: ctx.FlowContext(),
		Request: profileflow.EditRequest{
			Name:   runctx.FirstArg(args),
			Patch:  profilePatchFromFlags(cmd),
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
