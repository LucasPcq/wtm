package rules

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
)

// EffectiveAddressing is what a config actually runs with. Empty means names:
// a project that publishes no url is unaffected either way, and one that does
// publishes them so its apps can reach each other by name.
func EffectiveAddressing(cfg domain.RunConfig) domain.Addressing {
	if cfg.Addressing == domain.AddressingPorts {
		return domain.AddressingPorts
	}
	return domain.AddressingNames
}

// ValidateAddressing refuses a value that is neither of the two. A typo would
// otherwise read as the default and silently write the wrong thing into a .env.
func ValidateAddressing(cfg domain.RunConfig) []string {
	switch cfg.Addressing {
	case "", domain.AddressingPorts, domain.AddressingNames:
		return nil
	default:
		return []string{fmt.Sprintf("unknown addressing %q (expected %q or %q)",
			cfg.Addressing, domain.AddressingPorts, domain.AddressingNames)}
	}
}

// WorktreeCountLabel renders a worktree count for prose ("1 worktree" / "3 worktrees").
func WorktreeCountLabel(n int) string {
	if n == 1 {
		return fmt.Sprintf(domain.DashboardCountOneFmt, n)
	}
	return fmt.Sprintf(domain.DashboardCountFmt, n)
}

// BulkSettlesMain says whether a pass over every worktree may rewrite the main
// checkout's .env. Main is the one checkout that exists without wtm, the one a
// clone and a bare `docker compose up` read. A bulk pass may bring it back to
// ports, which is that state; moving it onto names makes it depend on the
// proxy, and is only ever done by naming it — `wtm env main`.
func BulkSettlesMain(mode domain.Addressing) bool {
	return mode == domain.AddressingPorts
}

type RunProxyPortParams struct {
	Run    domain.RunConfig
	Global domain.GlobalConfig
}

// RunProxyPort is the proxy port a run addresses its jobs through, zero when
// it hands out their own ports. The proxy is the machine's, the addressing the
// project's: under ports every .env spells localhost, so a name announced
// beside it would be one the apps' CORS origins and API urls know nothing of.
func RunProxyPort(params RunProxyPortParams) int {
	if EffectiveAddressing(params.Run) == domain.AddressingPorts {
		return 0
	}
	return ProxyPort(params.Global)
}
