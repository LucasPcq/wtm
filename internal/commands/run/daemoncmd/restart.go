package daemoncmd

import (
	"github.com/spf13/cobra"
	"io"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/process"
)

func newRestartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdRestart,
		Short: "Hand the jobs over to a daemon built from this binary",
		Long:  "Stop the running daemon and start one from this binary.\nThis is the way out of a version mismatch: the daemon is what runs the jobs, so an older one keeps serving its own behavior until it is replaced.\nDetached services keep running across the restart and are picked back up; foreground ones are stopped.",
		RunE:  runRestart,
	}
	shared.AddOutputFlag(cmd)
	cmd.Flags().BoolP(domain.FlagYes, "y", false, "Skip the confirmation")
	return cmd
}

func runRestart(cmd *cobra.Command, _ []string) error {
	status := collectStatus()
	if status.Running {
		confirmed, err := confirmStop(cmd, status)
		if err != nil {
			return err
		}
		if !confirmed {
			return domain.ErrUserAborted
		}
		if err := shutdown(); err != nil {
			return err
		}
	}

	global, err := config.LoadGlobal()
	if err != nil {
		return err
	}
	if err := process.EnsureDaemon(process.DaemonParams{
		SocketPath: process.SocketPath(),
		ProxyPort:  rules.ProxyPort(global),
	}); err != nil {
		return err
	}

	if format, _ := cmd.Flags().GetString(domain.FlagOutput); format == domain.OutputJSON {
		return output.WriteDaemonStatusJSON(cmd.OutOrStdout(), collectStatus())
	}
	output.Frame(cmd.OutOrStdout(), func(w io.Writer) {
		// The readout is the detail; without a conclusion above it the reader has
		// to infer the outcome from a `State` field, which every sibling states.
		output.Success(w, domain.DaemonRestarted)
		output.Blank(w)
		output.DaemonStatusFields(w, collectStatus())
	})
	return nil
}
