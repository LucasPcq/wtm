package daemoncmd

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

func newStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdStop,
		Short: "Stop the daemon, leaving detached services running",
		Long:  "Stop the background daemon.\nForeground services die with it — they are drained through a terminal it owns.\nDetached services (those with a stop command) keep running, and the next daemon picks them back up.",
		RunE:  runStop,
	}
	shared.AddOutputFlag(cmd)
	cmd.Flags().BoolP(domain.FlagYes, "y", false, "Skip the confirmation")
	return cmd
}

func runStop(cmd *cobra.Command, _ []string) error {
	status := collectStatus()
	if !status.Running {
		return reportStopped(cmd, domain.DaemonAlreadyStopped)
	}

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
	return reportStopped(cmd, domain.DaemonStopped)
}

// confirmStop only asks when there is something to lose. A daemon holding
// nothing but detached stacks costs nothing to stop — they outlive it either
// way — so asking would be a prompt with a single sensible answer.
func confirmStop(cmd *cobra.Command, status domain.DaemonStatus) (bool, error) {
	if status.Foreground == 0 {
		return true, nil
	}
	if yes, _ := cmd.Flags().GetBool(domain.FlagYes); yes {
		return true, nil
	}
	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	if format == domain.OutputJSON || !term.IsTerminal(int(os.Stdin.Fd())) {
		return false, fmt.Errorf("stopping the daemon would stop %d foreground service(s): pass --%s to confirm", status.Foreground, domain.FlagYes)
	}

	return components.RunStandaloneConfirm(components.NewConfirm(components.NewConfirmParams{
		Title:       domain.DaemonStopConfirmTitle,
		Description: fmt.Sprintf(domain.DaemonStopConfirmFmt, status.Foreground),
	}))
}

func shutdown() error {
	resp, err := process.NewClient(process.SocketPath()).SendUnchecked(process.Request{Action: process.ActionShutdown})
	if err != nil {
		return fmt.Errorf("stop daemon: %w", err)
	}
	if resp.Status == process.StatusError {
		return fmt.Errorf("stop daemon: %s", resp.Message)
	}
	return process.AwaitDaemonStopped(process.SocketPath())
}

func reportStopped(cmd *cobra.Command, message string) error {
	if format, _ := cmd.Flags().GetString(domain.FlagOutput); format == domain.OutputJSON {
		return output.WriteDaemonStatusJSON(cmd.OutOrStdout(), collectStatus())
	}
	output.Frame(cmd.OutOrStdout(), func(w io.Writer) {
		output.Success(w, message)
	})
	return nil
}
