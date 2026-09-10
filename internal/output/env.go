package output

import (
	"fmt"
	"io"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
)

// PrintEnvReport renders the per-file drift report of `wtm env`: one block per
// configured env file (its value source and the keys added / missing / in
// conflict / orphaned), then a trailing summary. It emits a raw body with no outer
// blank lines; the caller's frame owns the outer vertical padding. The blank
// between file blocks and before the summary is a genuine separator.
func PrintEnvReport(w io.Writer, result domain.EnvSyncResult) {
	writeAlignedFields(w, rules.EnvReportFields(result))
	Blank(w)

	for i, f := range result.Files {
		if i > 0 {
			Blank(w)
		}
		printEnvFile(w, f, result.Check, rules.EnvPortsMoveIn(rules.EnvPortsMoveInParams{Result: result, Target: f.Target}))
	}
	EnvPortsReport(w, result.Ports, result.Check)
	Blank(w)
	printEnvSummary(w, result)
}

// printEnvFile renders one file block: its header, then one aligned row per key.
// The glyph is a single rune rendered without a badge — a badge carries its own
// padding, which is what used to make the rows wander a column apart.
func printEnvFile(w io.Writer, f domain.EnvFileResult, check bool, hasPorts bool) {
	SectionTitle(w, fmt.Sprintf(domain.EnvFileHeaderFmt,
		f.Target,
		styles.Muted.Render(fmt.Sprintf(domain.EnvFileSourceFmt, f.Strategy, f.Source))))

	if f.Unresolvable {
		Warning(w, fmt.Sprintf(domain.EnvFileUnresolvableFmt, f.Target))
		return
	}

	rows := rules.EnvKeyRows(rules.EnvKeyRowsParams{File: f, Check: check})
	if len(rows) == 0 {
		Success(w, styles.Muted.Render(rules.EnvFileVerdict(hasPorts)))
		return
	}
	for _, row := range rows {
		printEnvKeyRow(w, row)
	}
}

// printEnvKeyRow prints one row of a file block. The glyphs are diff vocabulary
// — added, needs attention, left over — and never a tick: a tick states an
// outcome, and outcomes belong to the file verdict and the closing line.
func printEnvKeyRow(w io.Writer, row domain.EnvKeyRow) {
	glyph, text := styles.Success.Render(domain.EnvKeyGlyphAdd), row.Text
	switch row.Status {
	case domain.EnvKeyConflict, domain.EnvKeyMissing:
		glyph = styles.Warning.Render(domain.EnvKeyGlyphAttention)
	case domain.EnvKeyOrphan:
		glyph, text = styles.Muted.Render(domain.EnvKeyGlyphOrphan), styles.Muted.Render(text)
	}
	fmt.Fprintf(w, "%s%s %s\n", Indent, glyph, text)
}

// printEnvSummary prints the trailing one-line verdict in the register the rule
// gave it: `=` is what a run that had nothing to do says, and saying it over
// drift a --check run just found calls an open question a settled one.
func printEnvSummary(w io.Writer, result domain.EnvSyncResult) {
	summary := rules.EnvOutcomeSummary(result)
	switch summary.Verdict {
	case domain.EnvVerdictDone:
		Success(w, summary.Text)
	case domain.EnvVerdictAttention:
		Warning(w, summary.Text)
	default:
		Unchanged(w, summary.Text)
	}
}

// WriteEnvJSON writes the reconciliation result as pretty-printed JSON. The
// domain result carries its own json tags; it is never framed.
func WriteEnvJSON(w io.Writer, result domain.EnvSyncResult) error {
	return encodeJSON(w, result)
}
