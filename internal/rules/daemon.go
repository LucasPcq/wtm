package rules

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
)

type DaemonVersionMismatchParams struct {
	Client string
	Daemon string
}

// DaemonVersionMismatch words the refusal. It names both sides because the
// symptom of an old daemon is silence — the command reports what the new client
// believes while the old daemon does something else — so the message has to say
// what is actually running, and how to end it.
func DaemonVersionMismatch(params DaemonVersionMismatchParams) string {
	return fmt.Sprintf(domain.DaemonVersionMismatchFmt, DaemonVersionLabel(params.Daemon), params.Client)
}

// DaemonVersionMismatchLines is the same refusal for a callout, which boxes what
// it is given and so needs its own line breaks rather than a paragraph.
func DaemonVersionMismatchLines(params DaemonVersionMismatchParams) []string {
	return []string{
		fmt.Sprintf(domain.DaemonMismatchWhyFmt, DaemonVersionLabel(params.Daemon), params.Client),
		domain.DaemonMismatchReason,
		domain.DaemonMismatchFixLine,
	}
}

// IndexFrozenLines says what a read-only index costs, in the order a reader needs
// it: why it happened, what stops being recorded, and the way out.
func IndexFrozenLines(statePath string) []string {
	return []string{
		domain.DaemonIndexFrozenWhy,
		domain.DaemonIndexFrozenCost,
		fmt.Sprintf(domain.DaemonIndexFrozenFixFmt, statePath),
	}
}

// DaemonVersionDiverged reports a daemon this binary did not build. `status` is
// the one command that says so instead of refusing: it exists to be run when
// something else already refused.
func DaemonVersionDiverged(status domain.DaemonStatus) bool {
	return status.Running && status.DaemonVersion != status.Version
}

// DaemonVersionLabel names a daemon version for display, standing in for the
// build too old to stamp one.
func DaemonVersionLabel(version string) string {
	if version == "" {
		return domain.DaemonVersionUnknown
	}
	return version
}

// WorktreeJobsHaveErrors reports whether any job of any worktree refused the
// action, which is what decides whether the failures get a block of their own.
func WorktreeJobsHaveErrors(results []domain.WorktreeJobResults) bool {
	for _, worktree := range results {
		for _, job := range worktree.Jobs {
			if job.Status == domain.JobActionError {
				return true
			}
		}
	}
	return false
}
