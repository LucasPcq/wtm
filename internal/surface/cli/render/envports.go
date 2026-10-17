package render

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
	switch {
	case outcome == "":
	case check:
		Warning(w, outcome)
	default:
		Unchanged(w, outcome)
	}
	if len(anomalies) > 0 {
		Callout(w, domain.EnvPortAnomaliesTitle, anomalies)
	}
	for _, notice := range notices {
		Section(w, notice.Title, []string{notice.Line})
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
