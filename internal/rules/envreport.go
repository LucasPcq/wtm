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
// works through them: what was added, what is contested, what is unanswered,
// what is left over. The key column is padded across all four groups so the
// details line up — a block whose columns wander reads as noise next to the port
// table it sits beside.
//
// The glyph and the colour are the surface's to choose; a row only carries the
// status they derive from.
func EnvKeyRows(params EnvKeyRowsParams) []domain.EnvKeyRow {
	f := params.File
	added := resolvedEnvAdds(f.Diff)
	conflicts := EnvKeysWithStatus(EnvDiffFilter{Diff: f.Diff, Status: domain.EnvKeyConflict})
	missing := EnvKeysWithStatus(EnvDiffFilter{Diff: f.Diff, Status: domain.EnvKeyMissing})
	orphans := EnvKeysWithStatus(EnvDiffFilter{Diff: f.Diff, Status: domain.EnvKeyOrphan})

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
		rows = append(rows, row(e, fmt.Sprintf(addedDetailFmt(params), EnvSourceName(e.Source, f.ParentBranch))))
	}
	for _, e := range conflicts {
		rows = append(rows, row(e, fmt.Sprintf(domain.EnvDetailConflictFmt,
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

// addedDetailFmt tells apart the three moments an addition is reported: a
// read-only check, a write that happened, and a write still to come.
func addedDetailFmt(params EnvKeyRowsParams) string {
	switch {
	case params.Check:
		return domain.EnvDetailWouldAddFmt
	case params.File.Applied:
		return domain.EnvDetailAddedFmt
	default:
		return domain.EnvDetailToAddFmt
	}
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

// resolvedEnvAdds returns the resolved entries that are additions (absent from
// the child but backed by a real source value), the only resolved keys worth
// showing.
func resolvedEnvAdds(d domain.EnvDiff) []domain.EnvKeyDiff {
	out := make([]domain.EnvKeyDiff, 0)
	for _, e := range EnvKeysWithStatus(EnvDiffFilter{Diff: d, Status: domain.EnvKeyResolved}) {
		if e.CurrentValue == "" && e.ResolvedValue != "" {
			out = append(out, e)
		}
	}
	return out
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
