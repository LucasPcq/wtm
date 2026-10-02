package shared

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// EnvParentFallbackConfirm builds the confirmation prompt shown when the parent
// env strategy falls back to main for the given source branch.
func EnvParentFallbackConfirm(source string) components.NewConfirmParams {
	return components.NewConfirmParams{
		Title:      fmt.Sprintf(domain.EnvParentFallbackPrompt, source),
		Warning:    domain.EnvParentFallbackWarning,
		DefaultYes: true,
	}
}
