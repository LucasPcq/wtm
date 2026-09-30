package rules

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
)

type DetectedPortsSummaryParams struct {
	// Patched are the compose bindings templatized, Written the ports each job
	// gained in run.toml, and EnvWritten the ones the .env scan contributed.
	Patched    map[string][]domain.ComposePortBinding
	Written    map[string]map[string]int
	EnvWritten map[string]map[string]int
}

// DetectedPortsSummary is what the detection did, counted. It replaces the three
// lists that named every port and every binding: `run.toml` is the record of
// what was written, and a reader who wants the values opens it. What the
// detection *declined* to do is another matter and stays named one by one.
func DetectedPortsSummary(params DetectedPortsSummaryParams) string {
	declared := countPorts(params.Written) + countPorts(params.EnvWritten)
	if declared == 0 && len(params.Patched) == 0 {
		return ""
	}

	summary := fmt.Sprintf(domain.DetectedPortsSummaryFmt, declared)
	if len(params.Patched) > 0 {
		summary += fmt.Sprintf(domain.DetectedPortsPatchedFmt, len(params.Patched))
	}
	return summary
}

func countPorts(written map[string]map[string]int) int {
	total := 0
	for _, ports := range written {
		total += len(ports)
	}
	return total
}
