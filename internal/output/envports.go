package output

import (
	"fmt"
	"io"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// EnvPortsReport prints what the reader still has to act on after a port pass —
// the links wtm declined to act on, and what the machine made of the run — plus
// a single line for the pass itself.
//
// It lists no value. A settled port is a fact nobody decides on, and the table
// that named them all spent a screen restating what the .env beside it already
// holds; the count and the offset are the whole of what a reader checks.
//
// It emits a raw body with no surrounding blank lines; the caller's frame owns
// the padding. A plan with nothing to say prints nothing.
func EnvPortsReport(w io.Writer, plan domain.EnvPortPlan, check bool) {
	outcome := rules.EnvPortOutcomeLine(rules.EnvPortOutcomeParams{Plan: plan, Check: check})
	anomalies := rules.EnvPortAnomalyLines(plan)
	notices := rules.EnvPortNotices(plan)
	if outcome == "" && len(anomalies) == 0 && len(notices) == 0 {
		return
	}

	Blank(w)
	if outcome != "" {
		Unchanged(w, outcome)
	}
	printEnvPortAnomalies(w, anomalies)
	printEnvPortNotices(w, notices, outcome == "" && len(anomalies) == 0)
}

// printEnvPortNotices closes the section with what the machine, rather than any
// one value, made of the pass.
func printEnvPortNotices(w io.Writer, notices []rules.EnvPortNotice, alone bool) {
	for i, notice := range notices {
		if i > 0 || !alone {
			Blank(w)
		}
		Warning(w, notice.Title)
		Message(w, notice.Line)
	}
}

// printEnvPortAnomalies lists the links wtm refused to act on. They survive a
// declined pass: the user turned down the shift, not the news that a link never
// matches anything.
func printEnvPortAnomalies(w io.Writer, anomalies []string) {
	if len(anomalies) == 0 {
		return
	}
	Warning(w, domain.EnvPortAnomaliesTitle)
	for _, line := range anomalies {
		Message(w, line)
	}
}

// EnvPortLinksReport counts the links a `run init` just wrote. The links
// themselves are in run.toml, which the wizard proposed them from and which is
// the file a reader edits to change them.
func EnvPortLinksReport(w io.Writer, links []domain.EnvPortLink, bases map[domain.PortRef]int) {
	if len(links) == 0 {
		return
	}
	Blank(w)
	Success(w, fmt.Sprintf(domain.EnvPortLinksSummaryFmt, len(links)))
}

// PortKeysReport counts the ports a run just wrote into the project's env files.
func PortKeysReport(w io.Writer, writes []domain.PortKeyWrite) {
	if len(writes) == 0 {
		return
	}
	Blank(w)
	Success(w, fmt.Sprintf(domain.PortKeysSummaryFmt, len(writes)))
}
