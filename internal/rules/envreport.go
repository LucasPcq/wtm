package rules

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
)

// EnvReportFields is the context a reconciliation report opens with: which
// worktree it ran on, and in which mode. Both are in the result and neither was
// ever shown — a reader who omitted the argument and picked interactively had no
// confirmation of what had just been reconciled.
func EnvReportFields(result domain.EnvSyncResult) []domain.RecapField {
	mode := string(result.Mode)
	if result.Check {
		mode += domain.EnvModeCheckSuffix
	}

	fields := make([]domain.RecapField, 0, 3)
	if result.Branch != "" {
		fields = append(fields, domain.RecapField{Label: domain.EnvFieldWorktree, Value: result.Branch})
	}
	fields = append(fields, domain.RecapField{Label: domain.EnvFieldMode, Value: mode})
	// Only the exception is named: it is what explains a report in which no
	// port, identity or namespace was touched.
	if IsVerbatim(result.Isolation) {
		fields = append(fields, domain.RecapField{Label: domain.EnvFieldIsolation, Value: IsolationSummary(result.Isolation)})
	}
	return fields
}

type EnvKeyRowsParams struct {
	File  domain.EnvFileResult
	Check bool
}

// EnvKeyRows renders one file's keys as aligned rows, in the order a reader
// works through them: what is added, what is contested, what is unanswered,
// what is left over. A read-only check lists every key it would touch, since
// that list is what it was asked for; an apply lists only what it left for the
// reader — what it did is counted by EnvFileTally.
//
// The glyph and the colour are the surface's to choose; a row only carries the
// status they derive from.
func EnvKeyRows(params EnvKeyRowsParams) []domain.EnvKeyRow {
	f := params.File
	var added, conflicts, missing, orphans []domain.EnvKeyDiff
	for _, e := range f.Diff.Entries {
		if !params.Check && e.Action != domain.EnvActionKept && e.Action != "" {
			continue
		}
		switch {
		case e.Status == domain.EnvKeyResolved && envAddition(e) && params.Check:
			added = append(added, e)
		case e.Status == domain.EnvKeyConflict:
			conflicts = append(conflicts, e)
		case e.Status == domain.EnvKeyMissing:
			missing = append(missing, e)
		case e.Status == domain.EnvKeyOrphan:
			orphans = append(orphans, e)
		}
	}

	width := 0
	for _, group := range [][]domain.EnvKeyDiff{added, conflicts, missing, orphans} {
		for _, e := range group {
			width = max(width, len(e.Key))
		}
	}

	var rows []domain.EnvKeyRow
	row := func(e domain.EnvKeyDiff, detail string) domain.EnvKeyRow {
		return domain.EnvKeyRow{Status: e.Status, Text: pad(e.Key, width) + domain.EnvKeyRowGap + detail}
	}

	for _, e := range added {
		rows = append(rows, row(e, fmt.Sprintf(domain.EnvDetailWouldAddFmt, EnvSourceName(e.Source, f.ParentBranch))))
	}
	conflictFmt := domain.EnvDetailConflictFmt
	if !params.Check {
		conflictFmt = domain.EnvDetailConflictKeptFmt
	}
	for _, e := range conflicts {
		rows = append(rows, row(e, fmt.Sprintf(conflictFmt,
			EnvQuote(e.CurrentValue), EnvSourceName(e.Source, f.ParentBranch), EnvQuote(e.ResolvedValue))))
	}
	for _, e := range missing {
		rows = append(rows, row(e, fmt.Sprintf(domain.EnvDetailMissingFmt, EnvQuote(e.Placeholder))))
	}
	for _, e := range orphans {
		rows = append(rows, row(e, domain.EnvDetailOrphan))
	}
	return rows
}

// EnvFileTally counts what an apply did to a file's keys — "2 added · 1
// removed" — empty when it did nothing.
func EnvFileTally(file domain.EnvFileResult) string {
	counts := map[domain.EnvKeyAction]int{}
	for _, e := range file.Diff.Entries {
		counts[e.Action]++
	}
	return Tally(
		domain.TallyPart{Count: counts[domain.EnvActionAdded], Label: domain.EnvTallyAdded},
		domain.TallyPart{Count: counts[domain.EnvActionFilled], Label: domain.EnvTallyFilled},
		domain.TallyPart{Count: counts[domain.EnvActionOverwritten], Label: domain.EnvTallyOverwritten},
		domain.TallyPart{Count: counts[domain.EnvActionPruned], Label: domain.EnvTallyRemoved},
		domain.TallyPart{Count: counts[domain.EnvActionSkipped], Label: domain.EnvTallySkipped},
	)
}

type EnvFileVerdictParams struct {
	// PortsMove says the port pass rewrites a value in this very file.
	PortsMove bool
	Check     bool
}

// EnvFileVerdict is the single line a file block shows when no key needs
// anything. "In sync" alone would contradict a port pass that moves a value in
// this very file, and a pass that already ran is told in the past tense, or the
// block says its values still move under a summary counting them settled.
func EnvFileVerdict(params EnvFileVerdictParams) string {
	switch {
	case !params.PortsMove:
		return domain.EnvFileInSyncMessage
	case params.Check:
		return domain.EnvFileKeysInSyncMessage
	default:
		return domain.EnvFileValuesSettledMessage
	}
}

// EnvSourceName names the per-key cascade level: the actual parent branch when
// the value came from the parent worktree, else the level itself.
func EnvSourceName(level, parentBranch string) string {
	if level == domain.EnvSourceParent && parentBranch != "" {
		return parentBranch
	}
	return level
}

// EnvQuote renders a value for the report, naming the empty string so a blank
// value is visible rather than invisible.
func EnvQuote(v string) string {
	if v == "" {
		return domain.EnvEmptyValueLabel
	}
	return fmt.Sprintf("%q", v)
}
