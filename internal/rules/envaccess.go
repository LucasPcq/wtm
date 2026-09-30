package rules

import "github.com/LucasPcq/wtm/internal/domain"

// This file reads the env reconciliation types. It lives in rules/ rather than
// beside those types because internal/domain holds types, errors and constants
// only: a lookup over a domain value is a pure function like any other, and
// putting it on the type is how a layer starts growing behaviour.

type EnvDiffFilter struct {
	Diff   domain.EnvDiff
	Status domain.EnvKeyStatus
}

// EnvKeysWithStatus is the entries whose status matches, in the diff's own
// order — which is what makes a report read the way the file does.
func EnvKeysWithStatus(params EnvDiffFilter) []domain.EnvKeyDiff {
	out := make([]domain.EnvKeyDiff, 0, len(params.Diff.Entries))
	for _, entry := range params.Diff.Entries {
		if entry.Status == params.Status {
			out = append(out, entry)
		}
	}
	return out
}

// EnvDiffHasStatus reports whether any entry carries that status.
func EnvDiffHasStatus(params EnvDiffFilter) bool {
	for _, entry := range params.Diff.Entries {
		if entry.Status == params.Status {
			return true
		}
	}
	return false
}

// EnvPortRewrites is the entries of a plan that would change a value.
func EnvPortRewrites(plan domain.EnvPortPlan) []domain.EnvPortEntry {
	out := make([]domain.EnvPortEntry, 0, len(plan.Entries))
	for _, entry := range plan.Entries {
		if entry.Status == domain.EnvPortStatusRewrite {
			out = append(out, entry)
		}
	}
	return out
}

// EnvPortAnomalies is the entries wtm refuses to act on and reports instead.
func EnvPortAnomalies(plan domain.EnvPortPlan) []domain.EnvPortEntry {
	out := make([]domain.EnvPortEntry, 0, len(plan.Entries))
	for _, entry := range plan.Entries {
		switch entry.Status {
		case domain.EnvPortStatusMissingKey, domain.EnvPortStatusNotFound, domain.EnvPortStatusAmbiguous,
			domain.EnvPortStatusForeignHost, domain.EnvPortStatusSecureScheme:
			out = append(out, entry)
		}
	}
	return out
}

// EnvHasDrift reports whether the worktree is not fully reconciled: a key
// needing attention (missing, conflicting, orphaned) in any file, a linked
// value still carrying another worktree's port, or a key wtm writes in full
// still holding the value this worktree was copied from. Leaving any of them out
// would let `--check` answer "no drift" about a worktree whose .env points at
// the wrong services — or at another worktree's slice of a shared one.
func EnvHasDrift(result domain.EnvSyncResult) bool {
	for _, file := range result.Files {
		for _, status := range []domain.EnvKeyStatus{domain.EnvKeyMissing, domain.EnvKeyConflict, domain.EnvKeyOrphan} {
			if EnvDiffHasStatus(EnvDiffFilter{Diff: file.Diff, Status: status}) {
				return true
			}
		}
	}
	return len(EnvPortRewrites(result.Ports))+len(OwnedEnvRewrites(result.Ports)) > 0
}

// EnvAppliedFiles counts the env files whose reconciled content was written back.
func EnvAppliedFiles(result domain.EnvSyncResult) int {
	applied := 0
	for _, file := range result.Files {
		if file.Applied {
			applied++
		}
	}
	return applied
}
