package rules

import (
	"fmt"
	"slices"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// OwnedEnvKeyRefs are every .env key wtm writes into an isolated worktree:
// the [[env_port]] links, the [[env]] values and the identity keys of the files
// a compose stack reads.
func OwnedEnvKeyRefs(params OwnedEnvTargetsParams) []domain.EnvKeyRef {
	var refs []domain.EnvKeyRef
	add := func(ref domain.EnvKeyRef) {
		if !slices.Contains(refs, ref) {
			refs = append(refs, ref)
		}
	}
	for _, link := range params.Config.EnvPorts {
		add(domain.EnvKeyRef{File: link.File, Key: link.Key})
	}
	for _, link := range params.Config.EnvValues {
		add(domain.EnvKeyRef{File: link.File, Key: link.Key})
	}
	for _, target := range OwnedEnvTargets(params) {
		for _, key := range domain.WtmOwnedEnvKeys {
			add(domain.EnvKeyRef{File: target, Key: key})
		}
	}
	return refs
}

type RestoreOwnedEnvParams struct {
	File   string
	Child  []domain.EnvLine
	Source []domain.EnvLine
	Keys   []string
}

// RestoreOwnedEnv puts each owned key the child holds back to the source's
// value, and drops the ones the source does not have. A key the child lacks is
// left to the reconciliation, which adds it from the source like any other.
func RestoreOwnedEnv(params RestoreOwnedEnvParams) ([]domain.EnvLine, []domain.EnvRestoredEntry) {
	source := pairsByKey(params.Source)

	var entries []domain.EnvRestoredEntry
	out := make([]domain.EnvLine, 0, len(params.Child))
	for _, line := range params.Child {
		if line.Kind != domain.EnvLinePair || !slices.Contains(params.Keys, line.Key) {
			out = append(out, line)
			continue
		}
		from, present := source[line.Key]
		if !present {
			entries = append(entries, domain.EnvRestoredEntry{File: params.File, Key: line.Key, From: line.Value, Removed: true})
			continue
		}
		if from.Value != line.Value {
			entries = append(entries, domain.EnvRestoredEntry{File: params.File, Key: line.Key, From: line.Value, To: from.Value})
		}
		out = append(out, WithEnvValue(line, from.Value))
	}
	return out, entries
}

type EnvRestoredRowsParams struct {
	Entries    []domain.EnvRestoredEntry
	File       string
	ShowValues bool
}

// EnvRestoredRows renders one file's restored values as aligned rows. Without
// --show-values a row names only the origins wtm had moved: the value put back
// is the source's, the user's.
func EnvRestoredRows(params EnvRestoredRowsParams) []string {
	var mine []domain.EnvRestoredEntry
	width := 0
	for _, entry := range params.Entries {
		if entry.File == params.File {
			mine = append(mine, entry)
			width = max(width, len(entry.Key))
		}
	}

	rows := make([]string, 0, len(mine))
	for _, entry := range mine {
		rows = append(rows, pad(entry.Key, width)+domain.EnvKeyRowGap+restoredDetail(restoredDetailParams{Entry: entry, ShowValues: params.ShowValues}))
	}
	return rows
}

type restoredDetailParams struct {
	Entry      domain.EnvRestoredEntry
	ShowValues bool
}

func restoredDetail(params restoredDetailParams) string {
	entry := params.Entry
	if params.ShowValues {
		if entry.Removed {
			return fmt.Sprintf(domain.EnvDetailRestoredRemovedFmt, EnvQuote(entry.From))
		}
		return fmt.Sprintf(domain.EnvDetailRestoredFmt, EnvQuote(entry.To), EnvQuote(entry.From))
	}
	if entry.Removed {
		return domain.EnvDetailRestoredRemoved
	}
	moves, ok := EnvOriginMoves(EnvOriginMovesParams{From: entry.From, To: entry.To})
	if !ok {
		return domain.EnvDetailRestoredValue
	}
	return fmt.Sprintf(domain.EnvDetailRestoredFmt,
		strings.Join(originSide(moves, false), domain.EnvOriginJoin),
		strings.Join(originSide(moves, true), domain.EnvOriginJoin))
}
