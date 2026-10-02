package rules

import (
	"fmt"
	"slices"

	"github.com/LucasPcq/wtm/internal/domain"
)

// EnvResolvable reports whether a worktree's drift holds a key a reader can
// decide on; an in-sync worktree, or one with only owned values to settle, has
// none, and its resolve step is skipped.
func EnvResolvable(files []domain.EnvFileResult) bool {
	for _, file := range files {
		for _, entry := range file.Diff.Entries {
			if envDecidable(entry) {
				return true
			}
		}
	}
	return false
}

// EnvDriftCount is how many keys need a decision across a worktree's files.
func EnvDriftCount(files []domain.EnvFileResult) int {
	count := 0
	for _, file := range files {
		for _, entry := range file.Diff.Entries {
			switch entry.Status {
			case domain.EnvKeyConflict, domain.EnvKeyMissing, domain.EnvKeyOrphan:
				count++
			}
		}
	}
	return count
}

func envDecidable(entry domain.EnvKeyDiff) bool {
	switch entry.Status {
	case domain.EnvKeyConflict, domain.EnvKeyMissing, domain.EnvKeyOrphan:
		return true
	case domain.EnvKeyResolved:
		return envAddition(entry)
	}
	return false
}

func envAddition(entry domain.EnvKeyDiff) bool {
	return entry.CurrentValue == "" && entry.ResolvedValue != ""
}

type EnvResolveRecapParams struct {
	Files     []domain.EnvFileResult
	Decisions []domain.EnvFileDecision
}

// EnvResolveRecapLines restates every decidable key with what the apply does to
// it, grouped by file, in the order the resolver listed them.
func EnvResolveRecapLines(params EnvResolveRecapParams) []string {
	var lines []string
	for _, file := range params.Files {
		decision := envDecisionFor(params.Decisions, file.Target)
		var rows []string
		for _, entry := range file.Diff.Entries {
			if !envDecidable(entry) {
				continue
			}
			rows = append(rows, domain.RecapRowIndent+envRecapLine(entry, decision))
		}
		if len(rows) == 0 {
			continue
		}
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, fmt.Sprintf(domain.EnvRecapFileFmt, file.Target))
		lines = append(lines, rows...)
	}
	return lines
}

func envDecisionFor(decisions []domain.EnvFileDecision, target string) domain.EnvFileDecision {
	for _, decision := range decisions {
		if decision.Target == target {
			return decision
		}
	}
	return domain.EnvFileDecision{Target: target}
}

func envRecapLine(entry domain.EnvKeyDiff, decision domain.EnvFileDecision) string {
	line := func(action, value string) string {
		return fmt.Sprintf(domain.EnvRecapLineFmt, entry.Key, action, value)
	}
	if value, ok := decision.FilledValues[entry.Key]; ok {
		return line(domain.EnvRecapActionSet, envRecapValue(value))
	}
	switch entry.Status {
	case domain.EnvKeyConflict:
		if decision.Decisions[entry.Key] == domain.EnvDecisionOverwrite {
			return line(domain.EnvRecapActionOverwrite, envRecapValue(entry.ResolvedValue))
		}
	case domain.EnvKeyMissing:
		return line(domain.EnvRecapActionSkip, fmt.Sprintf(domain.EnvRecapNotAddedFmt, envRecapValue(entry.Placeholder)))
	case domain.EnvKeyOrphan:
		if slices.Contains(decision.PruneKeys, entry.Key) {
			return line(domain.EnvRecapActionRemove, envRecapValue(entry.CurrentValue))
		}
	case domain.EnvKeyResolved:
		if slices.Contains(decision.SkipKeys, entry.Key) {
			return line(domain.EnvRecapActionSkip, fmt.Sprintf(domain.EnvRecapNotAddedFmt, envRecapValue(entry.ResolvedValue)))
		}
		return line(domain.EnvRecapActionAdd, envRecapValue(entry.ResolvedValue))
	}
	return line(domain.EnvRecapActionKeep, envRecapValue(entry.CurrentValue))
}

func envRecapValue(value string) string {
	if value == "" {
		return domain.EnvRecapEmptyValue
	}
	return fmt.Sprintf("%q", value)
}

type EnvRestoreRecapParams struct {
	Entries []domain.EnvRestoredEntry
	// Switch is a run asked to go verbatim; otherwise the lines preview what the
	// verbatim action would do on top of the apply, and only when it is Offered.
	Switch  bool
	Offered bool
}

// EnvRestoreRecapLines previews what verbatim puts back, before it is written.
func EnvRestoreRecapLines(params EnvRestoreRecapParams) []string {
	if len(params.Entries) == 0 || (!params.Switch && !params.Offered) {
		return nil
	}
	title := domain.EnvRestoreRecapTitle
	if !params.Switch {
		title = domain.EnvRestoreRecapIfKeptTitle
	}

	lines := []string{"", title}
	var files []string
	for _, entry := range params.Entries {
		if !slices.Contains(files, entry.File) {
			files = append(files, entry.File)
		}
	}
	for _, file := range files {
		lines = append(lines, file)
		for _, row := range EnvRestoredRows(params.Entries, file) {
			lines = append(lines, domain.RecapRowIndent+row)
		}
	}
	return lines
}

// EnvPortRecapLines announces the port pass that rides along with the apply.
func EnvPortRecapLines(plan domain.EnvPortPlan) []string {
	table := EnvPortTableLines(EnvPortTableParams{Plan: plan})
	if len(table) == 0 {
		return nil
	}
	return append([]string{"", EnvPortOffsetLabel(plan.Offset)}, table...)
}
