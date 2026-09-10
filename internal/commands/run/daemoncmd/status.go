package daemoncmd

import (
	"github.com/spf13/cobra"
	"io"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/process"
)

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdStatus,
		Short: "Report whether a daemon is running, and which build it is",
		RunE:  runStatus,
	}
	shared.AddOutputFlag(cmd)
	return cmd
}

func runStatus(cmd *cobra.Command, _ []string) error {
	status := collectStatus()

	if format, _ := cmd.Flags().GetString(domain.FlagOutput); format == domain.OutputJSON {
		return output.WriteDaemonStatusJSON(cmd.OutOrStdout(), status)
	}
	output.Frame(cmd.OutOrStdout(), func(w io.Writer) {
		output.DaemonStatusReport(w, status)
	})
	return nil
}

// collectStatus asks the daemon rather than the index: the index says what was
// started, the daemon says what it is holding right now, and a status that
// disagreed with `run ps` would be worse than no status at all.
func collectStatus() domain.DaemonStatus {
	status := domain.DaemonStatus{
		SocketPath:  process.SocketPath(),
		StatePath:   process.StatePath(),
		Version:     domain.Version,
		IndexFrozen: process.IndexFrozen(),
	}
	if !process.IsDaemonRunning(status.SocketPath) {
		return status
	}

	// Sent raw: the client's own send refuses a version mismatch, which is the
	// one thing this command exists to report rather than hide.
	resp, err := process.NewClient(status.SocketPath).SendUnchecked(process.Request{Action: process.ActionList})
	if err != nil {
		return status
	}

	status.Running = true
	status.DaemonVersion = resp.Version
	status.PID = resp.DaemonPID
	status.ProxyPort = resp.ProxyPort
	for _, job := range resp.Jobs {
		if !rules.IsJobUp(job.Status) {
			continue
		}
		if job.Status == domain.JobStatusDetached {
			status.Detached++
			continue
		}
		status.Foreground++
	}
	return status
}
