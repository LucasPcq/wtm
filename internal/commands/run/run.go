package run

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/run/daemoncmd"
	"github.com/LucasPcq/wtm/internal/commands/run/jobcmd"
	"github.com/LucasPcq/wtm/internal/commands/run/profilecmd"
	"github.com/LucasPcq/wtm/internal/commands/run/proxycmd"
	"github.com/LucasPcq/wtm/internal/domain"
)

// NewCmd creates the wtm run command group — manages dev jobs
// (services + one-shot tasks) declared in run.toml.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdRun,
		Short: "Manage dev jobs (services + tasks)",
		Long: "Run commands and profiles declared in <git-common-dir>/wtm/run.toml — long-running services and one-shot tasks.\n\n" +
			"Vocabulary:\n" +
			"  job             the unit wtm runs; its kind is service (long-running) or task (one-shot)\n" +
			"  profile         a named, ordered group of jobs\n" +
			"  compose stack   a job that runs `docker compose`; an isolated worktree gets its own compose project\n" +
			"  shared service  a job with scope = \"shared\": one instance for the repository, run in\n" +
			"                  the main checkout; a worktree holding it reports it as joined\n" +
			"  namespace       a worktree's own part of a shared service — a database, a realm\n" +
			"  named URL       the address the run proxy serves (http://api.feat-x.myrepo.localhost)\n" +
			"  port URL        the job's own port (http://localhost:4012), printed with --raw\n" +
			"  foreign data    data this worktree does not own: its source's when it is verbatim,\n" +
			"                  every worktree's for a shared service with no namespace; a job whose\n" +
			"                  touches reach it is refused unless --force",
		GroupID: domain.CmdGroupJobs,
	}

	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newPsCmd())
	cmd.AddCommand(newURLCmd())
	cmd.AddCommand(newOpenCmd())
	cmd.AddCommand(newUpCmd())
	cmd.AddCommand(newDownCmd())
	cmd.AddCommand(newStartCmd())
	cmd.AddCommand(newStopCmd())
	cmd.AddCommand(newLogsCmd())
	cmd.AddCommand(newExportCmd())
	cmd.AddCommand(newImportCmd())
	cmd.AddCommand(jobcmd.NewCmd())
	cmd.AddCommand(profilecmd.NewCmd())
	cmd.AddCommand(newAddressingCmd())
	cmd.AddCommand(proxycmd.NewCmd())
	cmd.AddCommand(daemoncmd.NewCmd())

	return cmd
}
