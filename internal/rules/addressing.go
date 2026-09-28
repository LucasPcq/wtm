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
