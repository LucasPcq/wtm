package rules

import "github.com/LucasPcq/wtm/internal/domain"

type PublicPortParams struct {
	// BindPort is what the proxy really listens on, zero when it is off.
	BindPort int
	// Probed is what answered behind the privileged port, zero when nothing of
	// ours did. Only read when DaemonUp.
	Probed   int
	Declared bool
	DaemonUp bool
}

// PublicPort is the port a named URL announces. A running daemon settles it by
// probe; without one the installation's declared state answers, which is what
// lets a worktree being created write an origin with no daemon in sight.
func PublicPort(params PublicPortParams) int {
	if params.BindPort == 0 {
		return 0
	}
	if params.DaemonUp {
		if params.Probed == params.BindPort {
			return domain.ProxyPrivilegedPort
		}
		return params.BindPort
	}
	if params.Declared {
		return domain.ProxyPrivilegedPort
	}
	return params.BindPort
}
