package output

import (
	"fmt"
	"io"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

type DetectedPortsReportParams struct {
	// Patched are the rewrites that were applied, empty when none were.
	Patched map[string][]domain.ComposePortBinding
	// Written is what each job gained in run.toml.
	Written map[string]map[string]int
	// Withheld are the ports wtm detected but did not declare, and JobsByFile
	// names the job each one's file backs so the fix can be a command to paste.
	Withheld   []domain.ComposePortBinding
	JobsByFile map[string]string
	// Dropped are the detected declarations withdrawn to keep run.toml loadable.
	Dropped []rules.DroppedPort
	// Unreadable are the compose files the scan could not open or parse.
	Unreadable []domain.ComposeScan
	// Changed are the files that moved between the scan and the write, mapped to
	// the reason, and Orphaned those whose ports found no job to carry them.
	Changed  map[string]string
	Orphaned []string

	// EnvWritten is what the .env detection gave each job, EnvSources the file
	// each port came from, and EnvUnreadable the directories it could not read.
	EnvWritten    map[string]map[string]int
	EnvSources    map[string]map[string]string
	EnvUnreadable []domain.EnvPortScan
}

// DetectedPortsReport counts what the detection did and names, one by one, what
// it declined to do. The asymmetry is the design: a port written as expected is
// a fact run.toml already holds, while a binding left alone is a decision the
// reader still has to make, and only the second is worth a line.
//
// It emits a raw body with no surrounding blank lines; the caller's frame owns
// the padding. Nothing to say prints nothing.
func DetectedPortsReport(w io.Writer, params DetectedPortsReportParams) {
	if summary := rules.DetectedPortsSummary(rules.DetectedPortsSummaryParams{
		Patched:    params.Patched,
		Written:    params.Written,
		EnvWritten: params.EnvWritten,
	}); summary != "" {
		Blank(w)
		Success(w, summary)
	}

	if len(params.Withheld) > 0 {
		Blank(w)
		Callout(w, domain.ComposeWithheldTitle, withheldLines(params))
	}

	if len(params.Dropped) > 0 {
		Blank(w)
		lines := make([]string, 0, len(params.Dropped))
		for _, d := range params.Dropped {
			lines = append(lines, rules.ComposeDroppedLine(d))
		}
		Callout(w, domain.ComposeDroppedTitle, lines)
	}

	if len(params.Changed) > 0 {
		Blank(w)
		lines := make([]string, 0, len(params.Changed))
		for _, file := range rules.SortedComposeFiles(params.Changed) {
			lines = append(lines, fmt.Sprintf(domain.ComposeUnreadableFmt, file, params.Changed[file]))
		}
		Callout(w, domain.ComposeChangedTitle, lines)
	}

	if len(params.Orphaned) > 0 {
		Blank(w)
		lines := make([]string, 0, len(params.Orphaned))
		for _, file := range params.Orphaned {
			lines = append(lines, fmt.Sprintf(domain.ComposeOrphanFmt, file))
		}
		Callout(w, domain.ComposeOrphanTitle, lines)
	}

	for _, scan := range params.Unreadable {
		Blank(w)
		Warning(w, rules.ComposeUnreadableLine(scan))
	}

	for _, scan := range params.EnvUnreadable {
		Blank(w)
		Warning(w, scan.Err)
	}
}

func withheldLines(params DetectedPortsReportParams) []string {
	var lines []string
	for _, b := range params.Withheld {
		lines = append(lines, rules.ComposeWithheldLine(b))
		for _, fix := range rules.ComposeFixLines(rules.ComposeFixLinesParams{Binding: b, Job: params.JobsByFile[b.File]}) {
			lines = append(lines, fmt.Sprintf(domain.ComposeFixIndentFmt, fix))
		}
	}
	return lines
}
