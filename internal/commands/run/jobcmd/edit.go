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
			"--url-port, --url-host, --runs, --binds-no-port, --touches, --scope,\n" +
			"--namespace-name, --namespace-create, --namespace-remove or --namespace-env to\n" +
			"change those fields and nothing else: a flag left out keeps the field as it is,\n" +
			"and passing an empty string clears it (--stop '' drops the stop command,\n" +
			"--url-port '' withdraws the published name, --namespace-name '' withdraws the\n" +
			"whole [job.namespace]).\n\n" +
			"--scope shared runs one instance for the whole repository, in the main checkout;\n" +
			"--scope worktree puts it back to one per worktree. A [job.namespace] is only\n" +
			"accepted on a shared service and needs both a name and a create command, which\n" +
			"runs on every start and so must be safe to run again. --runs and --touches name\n" +
			"declared jobs; the file is refused exactly as loading it would refuse it.\n\n" +
			"--port merges into the ports the job already declares, so one entry can be\n" +
			"changed without rewriting the others; --port-clear empties the table.\n" +
			"--name also rewrites what names this job elsewhere in the file: the profiles,\n" +
			"the runners' runs, the touches, and the [[env_port]] and [[env]] links. It is\n" +
			"refused while a worktree holds data in the job, which clean finds by its name.\n\n" +
			"With no such flag, the form opens pre-filled with the current values, and\n" +
			"without an argument it prompts to pick from the existing jobs.",
		Args: cobra.MaximumNArgs(1),
		RunE: runEdit,
	}
	cmd.Flags().String(domain.FlagName, "", "Rename the job, updating the profiles, runs, touches, [[env_port]] and [[env]] links that name it")
	cmd.Flags().String(domain.FlagCmd, "", "Command to run, as a /bin/sh line")
	cmd.Flags().String(domain.FlagKind, "", "Job kind: service or task")
	cmd.Flags().String(domain.FlagStop, "", "Stop command, as a /bin/sh line (pass '' to drop it)")
	cmd.Flags().String(domain.FlagCwd, "", "Working directory relative to project root (pass '' to drop it)")
	cmd.Flags().StringArray(domain.FlagPort, nil, "Base port as NAME=PORT, repeatable — merged into the declared ports")
	cmd.Flags().Bool(domain.FlagPortClear, false, "Drop every port this job declares")
	cmd.Flags().String(domain.FlagURLPort, "", "Publish this declared port under a name (pass '' to withdraw the url)")
	cmd.Flags().String(domain.FlagURLHost, "", "Host segment to publish under (pass '' to fall back to the job's name)")
	addDeclarationFlags(cmd, editDeclarationHelp)
	shared.AddYesFlag(cmd, "Skip all prompts; a field flag is then required")
	shared.AddOutputFlag(cmd)
	return cmd
}

// jobPatchFromFlags reads the edit flags as a patch: only what the user
// actually passed, so an absent flag can be told from an explicit empty value.
func jobPatchFromFlags(cmd *cobra.Command) (rules.JobPatch, error) {
	patch := declarationPatch(cmd)
	patch.ClearPorts = changedBool(cmd, domain.FlagPortClear)
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
