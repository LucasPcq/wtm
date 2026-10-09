package daemon

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/process"
)

// NewCmd creates the hidden wtm daemon command.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    domain.CmdDaemon,
		Short:  "Run the service manager daemon (internal)",
		Hidden: true,
		RunE:   runDaemon,
	}
	cmd.Flags().Int(domain.FlagProxyPort, 0, "Port the run proxy serves named job URLs on (0 disables it)")
	return cmd
}

func runDaemon(cmd *cobra.Command, _ []string) error {
	proxyPort, err := resolveProxyPort(cmd)
	if err != nil {
		return err
	}
	return process.RunDaemon(process.DaemonParams{
		SocketPath: process.SocketPath(),
		ProxyPort:  proxyPort,
	})
}

// resolveProxyPort falls back to the global config when the flag is absent, so a
// daemon forked by a caller that could not read that config — `run down` from
// the flow layer, `clean`, `prune` — still serves the names the others publish.
// An explicit --proxy-port is honoured as given, zero included: that is how the
// proxy is turned off.
func resolveProxyPort(cmd *cobra.Command) (int, error) {
	if cmd.Flags().Changed(domain.FlagProxyPort) {
		port, _ := cmd.Flags().GetInt(domain.FlagProxyPort)
		return port, nil
	}
	global, err := config.LoadGlobal()
	if err != nil {
		return 0, err
	}
	return rules.ProxyPort(global), nil
}
