// Package branch decides where a branch stands: against its origin counterpart, and whether its parent can be inferred.
package branch

import (
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

func Divergence(ahead, behind int) domain.DivergenceState {
	switch {
	case ahead > 0 && behind > 0:
		return domain.DivergenceDiverged
	case behind > 0:
		return domain.DivergenceBehind
	case ahead > 0:
		return domain.DivergenceAhead
	}
	return domain.DivergenceUpToDate
}

// DivergenceLabel is "" for a branch with no origin counterpart, so a JSON field carrying it is omitted.
func DivergenceLabel(state domain.DivergenceState) string {
	switch state {
	case domain.DivergenceUpToDate:
		return domain.DivergenceLabelUpToDate
	case domain.DivergenceBehind:
		return domain.DivergenceLabelBehind
	case domain.DivergenceAhead:
		return domain.DivergenceLabelAhead
	case domain.DivergenceDiverged:
		return domain.DivergenceLabelDiverged
	case domain.DivergenceUnknown:
	}
	return ""
}

func IsRemote(name string) bool {
	return strings.HasPrefix(name, domain.RemoteBranchPrefix)
}

// ParentMustBeExplicit is a branch created outside wtm: nothing says what it was branched off, and a guessed parent would be read by sync and tree as fact.
func ParentMustBeExplicit(state domain.BranchTargetState) bool {
	return state == domain.BranchTargetExisting
}
