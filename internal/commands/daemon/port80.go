package daemon

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/service/proxy"
)

// NewProxyForwardCmd creates the hidden command launchd runs. It is not a user
// surface: launchd owns the privileged socket and hands it over, which is the
// whole reason wtm never needs a privilege of its own.
func NewProxyForwardCmd() *cobra.Command {
	return &cobra.Command{
		Use:    domain.CmdProxyForward,
		Short:  "Relay the launchd-owned privileged socket to the run proxy (internal)",
		Hidden: true,
		RunE:   runProxyForward,
	}
}

func runProxyForward(_ *cobra.Command, _ []string) error {
	listeners, err := proxy.LaunchdListeners(domain.ProxySocketKey)
	if err != nil {
		return err
	}

	return proxy.Forward(proxy.ForwardParams{
		Listeners: listeners,
		Target:    proxy.Cached(daemonProxyPort, time.Duration(domain.ProxyTargetCacheMs)*time.Millisecond),
	})
}

// daemonProxyPort asks the daemon where its proxy really is, rather than
// trusting a port frozen into the LaunchAgent at install time.
func daemonProxyPort() (int, error) {
	socketPath := process.SocketPath()
	if !process.IsDaemonRunning(socketPath) {
		return 0, domain.ErrProxyNoTarget
	}

	resp, err := process.NewClient(socketPath).Send(process.Request{Action: process.ActionList})
	if err != nil {
		return 0, err
	}
	if resp.ProxyPort == 0 {
		return 0, domain.ErrProxyNoTarget
	}
	return resp.ProxyPort, nil
}
