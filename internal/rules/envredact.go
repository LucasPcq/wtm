package rules

import (
	"slices"

	"github.com/LucasPcq/wtm/internal/domain"
)

// RedactEnvResult is a report without --show-values. It is an allow-list: of
// a value it keeps only what wtm wrote itself — an owned value whole, the
// host:port a port link moved — so a report piped into a log or an agent's
// context never carries a secret, whatever shape the value has. It returns a
// copy: the classification of a report runs on the values.
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

// AnnotateEnvOrigins adds to a full report the origins a redacted one keeps,
// so --show-values only adds to the JSON a reader already knows.
func AnnotateEnvOrigins(result domain.EnvSyncResult) domain.EnvSyncResult {
	result.Ports = annotatePortOrigins(result.Ports)
	result.Restored = annotateRestoredOrigins(result.Restored)
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
	redacted := annotateRestoredOrigins(entries)
	for i := range redacted {
		redacted[i].From = ""
		redacted[i].To = ""
	}
	return redacted
}

func annotateRestoredOrigins(entries []domain.EnvRestoredEntry) []domain.EnvRestoredEntry {
	annotated := slices.Clone(entries)
	for i, entry := range annotated {
		if entry.Removed {
			continue
		}
		annotated[i].Origins, _ = EnvOriginMoves(EnvOriginMovesParams{From: entry.From, To: entry.To})
	}
	return annotated
}

// RedactEnvPortPlan keeps of every port link the origins it moved, and drops
// the values they sit in.
func RedactEnvPortPlan(plan domain.EnvPortPlan) domain.EnvPortPlan {
	plan = annotatePortOrigins(plan)
	for i := range plan.Entries {
		plan.Entries[i].CurrentValue = ""
		plan.Entries[i].NewValue = ""
	}
	return plan
}

func annotatePortOrigins(plan domain.EnvPortPlan) domain.EnvPortPlan {
	entries := slices.Clone(plan.Entries)
	for i, entry := range entries {
		entries[i].Origins = EnvPortOrigins(entry)
	}
	plan.Entries = entries
	return plan
}

// EnvPortOrigins are the origins one link moves, none when it moves nothing.
func EnvPortOrigins(entry domain.EnvPortEntry) []domain.EnvOriginMove {
	if entry.NewValue == "" {
		return nil
	}
	moves, _ := EnvOriginMoves(EnvOriginMovesParams{From: entry.CurrentValue, To: entry.NewValue})
	return moves
}
