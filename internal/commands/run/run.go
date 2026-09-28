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
		Use:     domain.CmdRun,
		Short:   "Manage dev jobs (services + tasks)",
		Long:    "Run commands and profiles declared in <git-common-dir>/wtm/run.toml — long-running services and one-shot tasks.",
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
