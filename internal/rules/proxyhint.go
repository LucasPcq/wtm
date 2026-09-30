package rules

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
)

type ProxyInstallHintParams struct {
	Config     domain.RunConfig
	Status     domain.ProxyStatus
	ExampleURL string
}

// ProxyInstallHintLines is the one place wtm mentions the redirection outside
// its own commands: `run init` informs and never acts, so this returns lines,
// not a decision.
func ProxyInstallHintLines(params ProxyInstallHintParams) []string {
	if params.Status.Installed {
		return nil
	}
	if !anyJobPublishes(params.Config) {
		return nil
	}

	lines := []string{fmt.Sprintf(domain.ProxyInstallHintFmt, params.ExampleURL)}
	if !params.Status.Supported {
		return append(lines, domain.ProxyInstallHintNoPlat)
	}
	return append(lines, domain.ProxyInstallHintCmd)
}

func anyJobPublishes(cfg domain.RunConfig) bool {
	for _, job := range cfg.Jobs {
		if job.URL != nil {
			return true
		}
	}
	return false
}
