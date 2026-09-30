package process

import (
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/proxy"
)

// PublicProxyPort is the port a named URL announces on this machine. A running
// daemon has already probed what answers behind the privileged port, so its
// answer is taken whole; without one — which is most of the callers, since a
// worktree is often created with nothing running — the installation's declared
// state is all there is.
//
// Configured is the bind port the config asks for, zero when the proxy is off.
func PublicProxyPort(configured int) int {
	if configured == 0 {
		return 0
	}

	socketPath := SocketPath()
	if IsDaemonRunning(socketPath) {
		resp, err := NewClient(socketPath).Send(Request{Action: ActionList})
		if err == nil && resp.Status != StatusError {
			return resp.ProxyPublicPort
		}
	}
	return rules.PublicPort(rules.PublicPortParams{
		BindPort: configured,
		Declared: proxy.NewRedirector(proxy.RedirectorParams{}).Inspect().Installed,
	})
}
