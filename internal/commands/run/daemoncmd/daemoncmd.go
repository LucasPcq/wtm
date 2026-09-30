// Package daemoncmd implements `wtm run daemon status|stop|restart` — the way
// in and out of the process that actually runs the jobs. Before it, `pkill` was
// the only exit.
package daemoncmd

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
)

// NewCmd creates the wtm run daemon command group.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdDaemon,
		Short: "Inspect, stop or restart the process that runs the jobs",
		Long:  "Jobs are started by a background daemon shared by every repository.\nIt exits on its own once no foreground job is left; detached services keep running without it and are picked back up by the next one.",
		Example: `  wtm run daemon status
  wtm run daemon restart`,
	}

	cmd.AddCommand(newStatusCmd())
	cmd.AddCommand(newStopCmd())
	cmd.AddCommand(newRestartCmd())

	return cmd
}
