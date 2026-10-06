package rules

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
)

// FastForwardBlockers returns the refusals --force may lift. A dirty worktree is
// the only one: `git merge --ff-only` often succeeds on one, failing only when a
// modified file would be overwritten, so refusing is the safe default rather
// than the truth.
func FastForwardBlockers(check domain.FastForwardCheck) []domain.FastForwardBlocker {
	if _, refused := FastForwardRefusal(check); refused {
		return nil
	}
	if !check.IsDirty {
		return nil
	}
	return []domain.FastForwardBlocker{{
		Key:   domain.FastForwardBlockerDirty,
		Label: domain.FastForwardWarnDirty,
	}}
}

// FastForwardRefusal returns the refusals nothing lifts. Advancing a diverged
// branch onto origin means dropping its local commits, which is a reset and not
// a fast-forward; keeping it out of FastForwardBlockers is what leaves --force
// structurally unable to reach it.
func FastForwardRefusal(check domain.FastForwardCheck) (string, bool) {
	if !check.HasUpstream {
		return fmt.Sprintf(domain.FastForwardNoRemoteFmt, check.Branch), true
	}
	if check.State == domain.DivergenceDiverged {
		return fmt.Sprintf(domain.FastForwardDivergedHintFmt, check.Branch, check.Ahead, check.Behind, check.Branch), true
	}
	return "", false
}

// FastForwardNeedsWork reports whether the branch would actually move.
func FastForwardNeedsWork(check domain.FastForwardCheck) bool {
	if _, refused := FastForwardRefusal(check); refused {
		return false
	}
	return check.State == domain.DivergenceBehind
}

// FastForwardReadyBranches is what a batch run arrives with checked: the
// worktrees the cached badges call behind. A dirty one stays in — the recap
// refuses it by name, and dropping it here would hide a branch that is behind.
func FastForwardReadyBranches(statuses []domain.WorktreeStatus) []string {
	branches := make([]string, 0, len(statuses))
	for _, status := range statuses {
		if status.OriginState != domain.DivergenceBehind {
			continue
		}
		branches = append(branches, status.Branch)
	}
	return branches
}

func FastForwardStatusLabel(status domain.FastForwardStatus) string {
	switch status {
	case domain.FFAdvanced:
		return domain.FastForwardLabelAdvanced
	case domain.FFDiverged:
		return domain.FastForwardLabelDiverged
	case domain.FFNoUpstream:
		return domain.FastForwardLabelNoRemote
	case domain.FFFailed:
		return domain.FastForwardLabelFailed
	case domain.FFCancelled:
		return domain.FastForwardLabelCancelled
	default:
		return domain.FastForwardLabelUpToDate
	}
}

// FastForwardInterrupted reports a run an interrupt stopped before every branch.
func FastForwardInterrupted(results []domain.FastForwardResult) bool {
	for _, result := range results {
		if result.Status == domain.FFCancelled {
			return true
		}
	}
	return false
}
