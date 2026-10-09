package proxycmd

import (
	"context"
	"io"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/service/proxy"
	"github.com/LucasPcq/wtm/internal/surface/cli/render"
	"github.com/LucasPcq/wtm/internal/surface/cli/shared"
)

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdStatus,
		Short: "Report what actually serves named URLs on this machine",
		Example: `  wtm run proxy status

  wtm run proxy status --output json`,
		RunE: runStatus,
	}
	shared.AddOutputFlag(cmd)
	return cmd
}

func runStatus(cmd *cobra.Command, _ []string) error {
	configured, err := configuredBindPort()
	if err != nil {
		return err
	}

	status := collectStatus(cmd.Context(), configured)

	if format, _ := cmd.Flags().GetString(domain.FlagOutput); format == domain.OutputJSON {
		return render.WriteProxyStatusJSON(cmd.OutOrStdout(), status)
	}
	render.Frame(cmd.OutOrStdout(), func(w io.Writer) {
		render.ProxyStatusReport(w, status)
	})
	return nil
}

// configuredBindPort reads the global config alone: the proxy's port and its
// redirection belong to the machine, so `run proxy` answers outside a project.
func configuredBindPort() (int, error) {
	global, err := config.LoadGlobal()
	if err != nil {
		return 0, err
	}
	return rules.ProxyPort(global), nil
}

func collectStatus(ctx context.Context, configured int) domain.ProxyStatus {
	status := proxy.NewRedirector(proxy.RedirectorParams{}).Inspect()
	status.ConfiguredPort = configured
	status.BindPort = configured
	status.ConfigPath = config.GlobalPath()

	socketPath := process.SocketPath()
	if process.IsDaemonRunning(socketPath) {
		resp, err := process.NewClient(socketPath).Send(ctx, process.Request{Action: process.ActionList})
		if err == nil && resp.Status != process.StatusError {
			status.DaemonUp = true
			status.BindPort = resp.ProxyPort
			status.Probed = proxy.Probe(proxy.ProbeParams{Port: domain.ProxyPrivilegedPort})
		}
	}

	status.PublicPort = rules.PublicPort(rules.PublicPortParams{
		BindPort: status.BindPort,
		Probed:   status.Probed,
		Declared: status.Installed,
		DaemonUp: status.DaemonUp,
	})
	// Declared but not reaching us: a pf flush, an OS update rewriting
	// /etc/pf.conf, or a bind port that moved since install.
	status.Diverged = status.Installed && status.DaemonUp && status.PublicPort != domain.ProxyPrivilegedPort
	return status
}
