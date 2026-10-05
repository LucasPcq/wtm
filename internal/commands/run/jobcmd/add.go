package jobcmd

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/run/runctx"
	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	jobflow "github.com/LucasPcq/wtm/internal/flow/run/job"
	"github.com/LucasPcq/wtm/internal/rules"
)

func newAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdAdd + " [name]",
		Short: "Add a job to run.toml",
		Long: "Append a job to <git-common-dir>/wtm/run.toml.\n\n" +
			"Every flag pre-fills the corresponding question, so the form opens on what was\n" +
			"already given. --yes skips the questions altogether: [name] and --cmd are then\n" +
			"required, and every other field falls back to its documented default.\n\n" +
			"--runs, --touches and --binds-no-port declare how the job relates to the others;\n" +
			"--scope shared and the --namespace-* flags declare a service run once for the\n" +
			"whole repository and each worktree's namespace in it. The file is refused exactly\n" +
			"as loading it would refuse it: a namespace only on a shared service, with both a\n" +
			"name and a create command; --runs and --touches naming declared jobs.\n\n" +
			"--cmd and --stop are /bin/sh lines: quotes, && and ${VAR} behave as in a terminal,\n" +
			"so a declared port can be passed as a flag — --cmd 'pnpm dev --port ${PORT}'.",
		Example: `  # Answer the form
  wtm run job add

  # A dev server with its own port per worktree and a named URL
  wtm run job add web --cmd 'pnpm dev --port ${PORT}' --cwd apps/web --port PORT=3000 --url-port PORT --yes

  # A migration, which changes the data of the postgres job
  wtm run job add migrate --kind task --cmd 'pnpm db:migrate' --touches postgres --yes

  # One postgres for the repository, a database per worktree
  wtm run job add postgres --cmd 'docker compose up -d postgres' --stop 'docker compose stop postgres' \
    --scope shared --port POSTGRES_PORT=5432 --namespace-name 'app_{worktree}' \
    --namespace-create scripts/db-add.sh --namespace-remove scripts/db-drop.sh --yes`,
		Args: cobra.MaximumNArgs(1),
		RunE: runAdd,
	}
	cmd.Flags().String(domain.FlagCmd, "", "Command to run, as a /bin/sh line")
	cmd.Flags().String(domain.FlagKind, string(domain.JobKindService), "Job kind: service or task")
	cmd.Flags().String(domain.FlagStop, "", "Stop command, as a /bin/sh line (services only)")
	cmd.Flags().String(domain.FlagCwd, "", "Working directory (relative to project root)")
	cmd.Flags().StringArray(domain.FlagPort, nil, "Base port as NAME=PORT, repeatable (e.g. --port PORT=3000)")
	cmd.Flags().String(domain.FlagURLPort, "", "Publish this declared port under a name (e.g. --url-port PORT)")
	cmd.Flags().String(domain.FlagURLHost, "", "Host segment to publish under, defaulting to the job's name")
	addDeclarationFlags(cmd, addDeclarationHelp)
	shared.AddYesFlag(cmd, "Skip all prompts; [name] and --cmd are then required")
	shared.AddOutputFlag(cmd)
	return cmd
}

func runAdd(cmd *cobra.Command, args []string) error {
	// The creation path: a repository whose run.toml declares nothing yet is
	// exactly what this command is for, so the opt-in guard does not apply.
	ctx, err := runctx.Open(runctx.OpenParams{Cmd: cmd, SkipGuard: true})
	if err != nil {
		return err
	}

	initial, err := jobFromFlags(cmd, args)
	if err != nil {
		return err
	}

	outcome, err := jobflow.Add(cmd.Context(), jobflow.AddParams{
		Context:   ctx.FlowContext(),
		Request:   jobflow.AddRequest{Initial: initial, Config: ctx.Run},
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

func jobFromFlags(cmd *cobra.Command, args []string) (domain.JobConfig, error) {
	portFlags, _ := cmd.Flags().GetStringArray(domain.FlagPort)
	ports, err := rules.ParsePorts(portFlags)
	if err != nil {
		return domain.JobConfig{}, err
	}
	port, _ := cmd.Flags().GetString(domain.FlagURLPort)
	host, _ := cmd.Flags().GetString(domain.FlagURLHost)
	jobURL, err := rules.JobURLFromFlags(rules.JobURLFlagsParams{Port: port, Host: host})
	if err != nil {
		return domain.JobConfig{}, err
	}

	kind, _ := cmd.Flags().GetString(domain.FlagKind)
	cmdLine, _ := cmd.Flags().GetString(domain.FlagCmd)
	stop, _ := cmd.Flags().GetString(domain.FlagStop)
	cwd, _ := cmd.Flags().GetString(domain.FlagCwd)

	var name string
	if len(args) > 0 {
		name = args[0]
	}
	return rules.ApplyJobPatch(rules.ApplyJobPatchParams{
		Current: domain.JobConfig{
			Name:  name,
			Kind:  domain.JobKind(kind),
			Cmd:   cmdLine,
			Stop:  stop,
			Cwd:   cwd,
			Ports: ports,
			URL:   jobURL,
		},
		Patch: declarationPatch(cmd),
	})
}
