package rules

import (
	"slices"

	"github.com/LucasPcq/wtm/internal/domain"
)

// RedactEnvResult is a report without --show-values. No text read from a
// value reaches it: what stays is what wtm builds itself — an owned value it
// rendered, a link's ports and the address it writes — so a report piped into
// a log or an agent's context never carries a secret, whatever shape the value
// has. It returns a copy: the classification of a report runs on the values.
//
// The one value that stays is a missing key's placeholder: it is read from the
// committed template, which is meant to be read, and it is what the reader has
// to fill in.
func RedactEnvResult(result domain.EnvSyncResult) domain.EnvSyncResult {
	files := slices.Clone(result.Files)
	for i, file := range files {
		entries := slices.Clone(file.Diff.Entries)
		for j, entry := range entries {
			entries[j] = redactEnvKey(entry)
		}
		files[i].Diff.Entries = entries
	}
	result.Files = files
	result.Ports = RedactEnvPortPlan(result.Ports)
	result.Restored = redactEnvRestored(result.Restored)
	return result
}

// redactEnvKey withholds every value of the reconciliation, the managed keys'
// included: the values it compares are read from the files, so they are the
// user's, and what wtm writes is in the port plan.
func redactEnvKey(entry domain.EnvKeyDiff) domain.EnvKeyDiff {
	if entry.CurrentValue == "" && entry.ResolvedValue == "" {
		return entry
	}
	entry.CurrentValue = ""
	entry.ResolvedValue = ""
	entry.Redacted = true
	return entry
}

func redactEnvRestored(entries []domain.EnvRestoredEntry) []domain.EnvRestoredEntry {
	redacted := slices.Clone(entries)
	for i := range redacted {
		redacted[i].From = ""
		redacted[i].To = ""
	}
	return redacted
}

// RedactEnvPortPlan drops from every port link the values it was read from
// and written to, and the host a refusal found in them; the moves, numbers and
// addresses wtm builds, say the rest.
func RedactEnvPortPlan(plan domain.EnvPortPlan) domain.EnvPortPlan {
	entries := slices.Clone(plan.Entries)
	for i := range entries {
		entries[i].CurrentValue = ""
		entries[i].NewValue = ""
		entries[i].ForeignHost = ""
	}
	plan.Entries = entries
	return plan
}
