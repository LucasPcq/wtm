package rules

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
)

type EnvFlagsParams struct {
	Check         bool
	Prune         bool
	OnConflictSet bool
	Mode          domain.EnvMode
	Isolation     domain.Isolation
}

// ValidateEnvFlags refuses a combination `wtm env` could only honour by
// ignoring part of it. A read-only --check never prompts, so it needs no --yes
// even in JSON.
func ValidateEnvFlags(params EnvFlagsParams) error {
	switch {
	case params.Check && params.Isolation != "":
		return domain.ErrEnvIsolationWithCheck
	case params.Check && params.Prune:
		return fmt.Errorf(domain.EnvDecisionWithCheckFmt, domain.FlagPrune, domain.ErrEnvDecisionWithCheck)
	case params.Check && params.OnConflictSet:
		return fmt.Errorf(domain.EnvDecisionWithCheckFmt, domain.FlagOnConflict, domain.ErrEnvDecisionWithCheck)
	case params.OnConflictSet && params.Mode == domain.EnvModeAdd:
		return domain.ErrEnvOnConflictNeedsRefresh
	}
	return nil
}
