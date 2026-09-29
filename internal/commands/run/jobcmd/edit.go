package jobcmd

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/run/runctx"
	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	jobflow "github.com/LucasPcq/wtm/internal/flow/run/job"
	"github.com/LucasPcq/wtm/internal/rules"
)

func newEditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdEdit + " [name]",
		Short: "Edit an existing job",
		Long: "Edit a job declared in <git-common-dir>/wtm/run.toml.\n\n" +
			"Pass any of --name, --cmd, --kind, --stop, --cwd, --port, --port-clear,\n" +
			"--url-port or --url-host to change those fields and nothing else: a flag left\n" +
			"out keeps the field as it is, and passing an empty string clears it (--stop ''\n" +
			"drops the stop command, --url-port '' withdraws the published name).\n\n" +
			"--port merges into the ports the job already declares, so one entry can be\n" +
			"changed without rewriting the others; --port-clear empties the table.\n" +
			"--name also rewrites what names this job elsewhere in the file: the profiles\n" +
			"that start it and the env_port links that follow its ports.\n\n" +
			"With no such flag, the form opens pre-filled with the current values, and\n" +
			"without an argument it prompts to pick from the existing jobs.",
		Args: cobra.MaximumNArgs(1),
		RunE: runEdit,
	}
	cmd.Flags().String(domain.FlagName, "", "Rename the job, updating the profiles and env_port links that name it")
	cmd.Flags().String(domain.FlagCmd, "", "Command to run, as a /bin/sh line")
	cmd.Flags().String(domain.FlagKind, "", "Job kind: service or task")
	cmd.Flags().String(domain.FlagStop, "", "Stop command, as a /bin/sh line (pass '' to drop it)")
	cmd.Flags().String(domain.FlagCwd, "", "Working directory relative to project root (pass '' to drop it)")
	cmd.Flags().StringArray(domain.FlagPort, nil, "Base port as NAME=PORT, repeatable — merged into the declared ports")
	cmd.Flags().Bool(domain.FlagPortClear, false, "Drop every port this job declares")
	cmd.Flags().String(domain.FlagURLPort, "", "Publish this declared port under a name (pass '' to withdraw the url)")
	cmd.Flags().String(domain.FlagURLHost, "", "Host segment to publish under (pass '' to fall back to the job's name)")
	cmd.Flags().StringArray(domain.FlagRuns, nil, "Declared job this one starts itself, repeatable — replaces the list (pass '' to drop it)")
	cmd.Flags().Bool(domain.FlagBindsNoPort, false, "This service listens on nothing by design, so stop offering it a port")
	cmd.Flags().StringArray(domain.FlagTouches, nil, "Declared service whose data this job changes (a migration, a reset, a seed), repeatable — replaces the list (pass '' to drop it)")
	shared.AddYesFlag(cmd, "Skip all prompts; a field flag is then required")
	shared.AddOutputFlag(cmd)
	return cmd
}

// jobPatchFromFlags reads the edit flags as a patch: only what the user
// actually passed, so an absent flag can be told from an explicit empty value.
func jobPatchFromFlags(cmd *cobra.Command) (rules.JobPatch, error) {
	patch := rules.JobPatch{ClearPorts: changedBool(cmd, domain.FlagPortClear)}
	for _, field := range []struct {
		flag string
		into **string
	}{
		{domain.FlagName, &patch.Name},
		{domain.FlagCmd, &patch.Cmd},
		{domain.FlagKind, &patch.Kind},
		{domain.FlagStop, &patch.Stop},
		{domain.FlagCwd, &patch.Cwd},
		{domain.FlagURLPort, &patch.URLPort},
		{domain.FlagURLHost, &patch.URLHost},
	} {
		if !cmd.Flags().Changed(field.flag) {
			continue
		}
		value, _ := cmd.Flags().GetString(field.flag)
		*field.into = &value
	}

	if cmd.Flags().Changed(domain.FlagRuns) {
		runs, _ := cmd.Flags().GetStringArray(domain.FlagRuns)
		patch.Runs = &runs
	}
	if cmd.Flags().Changed(domain.FlagTouches) {
		touches, _ := cmd.Flags().GetStringArray(domain.FlagTouches)
		patch.Touches = &touches
	}
	if cmd.Flags().Changed(domain.FlagBindsNoPort) {
		binds, _ := cmd.Flags().GetBool(domain.FlagBindsNoPort)
		patch.BindsNoPort = &binds
	}

	portFlags, _ := cmd.Flags().GetStringArray(domain.FlagPort)
	ports, err := rules.ParsePorts(portFlags)
	if err != nil {
		return rules.JobPatch{}, err
	}
	patch.Ports = ports

	return patch, nil
}

func changedBool(cmd *cobra.Command, flag string) bool {
	if !cmd.Flags().Changed(flag) {
		return false
	}
	value, _ := cmd.Flags().GetBool(flag)
	return value
}

func runEdit(cmd *cobra.Command, args []string) error {
	ctx, err := runctx.Open(runctx.OpenParams{Cmd: cmd})
	if err != nil {
		return err
	}

	patch, err := jobPatchFromFlags(cmd)
	if err != nil {
		return err
	}

	outcome, err := jobflow.Edit(jobflow.EditParams{
		Context: ctx.FlowContext(),
		Request: jobflow.EditRequest{
			Name:   runctx.FirstArg(args),
			Patch:  patch,
			Config: ctx.Run,
		},
		Prompter:  ctx.Prompter(ctx.Interactive),
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
