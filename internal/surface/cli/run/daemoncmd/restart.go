package daemoncmd

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/surface/cli/render"
	"github.com/LucasPcq/wtm/internal/surface/cli/shared"
)

func newRestartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdRestart,
		Short: "Hand the jobs over to a daemon built from this binary",
		Long:  "Stop the running daemon and start one from this binary.\nThis is the way out of a version mismatch: the daemon is what runs the jobs, so an older one keeps serving its own behavior until it is replaced.\nDetached services keep running across the restart and are picked back up; foreground ones are stopped.",
		Example: `  # After an upgrade, when a run command reports the daemon's version
  wtm run daemon restart

  wtm run daemon restart --yes`,
		RunE: runRestart,
	}
	shared.AddOutputFlag(cmd)
	cmd.Flags().BoolP(domain.FlagYes, "y", false, "Skip the confirmation")
	return cmd
}

func runRestart(cmd *cobra.Command, _ []string) error {
	status := collectStatus(cmd.Context())
	if status.Running {
		confirmed, err := confirmStop(cmd, status)
		if err != nil {
			return err
		}
		if !confirmed {
			return domain.ErrUserAborted
		}
		if err := shutdown(cmd.Context()); err != nil {
			return err
		}
	}

	global, err := config.LoadGlobal()
	if err != nil {
		return err
	}
	if err := process.EnsureDaemon(cmd.Context(), process.DaemonParams{
		SocketPath: process.SocketPath(),
		ProxyPort:  rules.ProxyPort(global),
	}); err != nil {
		return err
	}

	if format, _ := cmd.Flags().GetString(domain.FlagOutput); format == domain.OutputJSON {
		return render.WriteDaemonStatusJSON(cmd.OutOrStdout(), collectStatus(cmd.Context()))
	}
	render.Frame(cmd.OutOrStdout(), func(w io.Writer) {
		// The readout is the detail; without a conclusion above it the reader has
		// to infer the outcome from a `State` field, which every sibling states.
		render.Success(w, domain.DaemonRestarted)
		render.Blank(w)
		render.DaemonStatusFields(w, collectStatus(cmd.Context()))
	})
	return nil
}
