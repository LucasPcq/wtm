package output

import (
	"fmt"
	"io"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
)

// PrintEnvReport renders the report of `wtm env` in the shape of an act: the
// verdict first, the worktree and the mode, then one block per configured env
// file — what the run did to it counted, what it left for the reader named key
// by key — and the port pass. It emits a raw body with no outer blank lines;
// the caller's frame owns the outer padding.
func PrintEnvReport(w io.Writer, result domain.EnvSyncResult) {
	printEnvSummary(w, result)
	Blank(w)
	writeAlignedFields(w, rules.EnvReportFields(result))

	for _, f := range result.Files {
		Blank(w)
		printEnvFile(w, envFileBlock{
			file:     f,
			check:    result.Check,
			hasPorts: rules.EnvPortsMoveIn(rules.EnvPortsMoveInParams{Result: result, Target: f.Target}),
			restored: rules.EnvRestoredRows(result.Restored, f.Target),
		})
	}
	EnvPortsReport(w, result.Ports, result.Check)
}

type envFileBlock struct {
	file     domain.EnvFileResult
	check    bool
	hasPorts bool
	restored []string
}

// printEnvFile renders one file block: its header, what the run did as one
// counted line, then one aligned row per key it left for the reader.
func printEnvFile(w io.Writer, block envFileBlock) {
	f, check := block.file, block.check
	SectionTitle(w, fmt.Sprintf(domain.EnvFileHeaderFmt,
		f.Target,
		styles.Muted.Render(fmt.Sprintf(domain.EnvFileSourceFmt, f.Strategy, f.Source))))

	if f.Unresolvable {
		Warning(w, fmt.Sprintf(domain.EnvFileUnresolvableFmt, f.Target))
		return
	}

	tally := ""
	if !check {
		tally = rules.EnvFileTally(f)
	}
	if tally != "" {
		Success(w, tally)
	}
	for _, row := range block.restored {
		Update(w, row)
	}
	rows := rules.EnvKeyRows(rules.EnvKeyRowsParams{File: f, Check: check})
	for _, row := range rows {
		printEnvKeyRow(w, row)
	}
	if tally != "" || len(rows) > 0 || len(block.restored) > 0 {
		return
	}

	verdict := rules.EnvFileVerdict(rules.EnvFileVerdictParams{PortsMove: block.hasPorts, Check: check})
	switch {
	case !block.hasPorts:
		Unchanged(w, verdict)
	case check:
		Warning(w, verdict)
	default:
		Success(w, verdict)
	}
}

// printEnvKeyRow prints one row of a file block. The glyphs are diff vocabulary
// — to add, needs attention, left over — and never a tick: a tick states an
// outcome, and outcomes belong to the counted line and the verdict.
func printEnvKeyRow(w io.Writer, row domain.EnvKeyRow) {
	glyph := styles.Success.Render(domain.EnvKeyGlyphAdd)
	switch row.Status {
	case domain.EnvKeyConflict, domain.EnvKeyMissing:
		glyph = styles.Warning.Render(domain.EnvKeyGlyphAttention)
	case domain.EnvKeyOrphan:
		glyph = styles.Muted.Render(domain.EnvKeyGlyphOrphan)
	}
	fmt.Fprintf(w, "%s%s %s\n", Indent, glyph, row.Text)
}

// printEnvSummary prints the verdict in the register the rule gave it: `=` is
// what a run that had nothing to do says, and saying it over drift a --check
// run just found calls an open question a settled one.
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
