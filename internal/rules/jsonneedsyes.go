package rules

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
)

// JSONNeedsYes refuses --output json on a command that could ask, naming every
// flag that would have let it run unattended.
func JSONNeedsYes(flags []string) error {
	spelled := make([]string, 0, len(flags))
	for _, flag := range flags {
		spelled = append(spelled, "--"+flag)
	}
	return flagValueError{text: fmt.Sprintf(domain.JSONNeedsYesFmt, alternatives(spelled))}
}
