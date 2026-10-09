package output

import (
	"io"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

type PortIsolationReportParams struct {
	// Unported are the services that declared no port at all.
	Unported []string
	// Ignoring are the services whose command never mentions the port they were
	// given.
	Ignoring []domain.JobCmdFix
}

// PortIsolationReport is the one thing the reader still has to act on: the jobs
// that will bind the same port in every worktree. Both halves say that, so they
// share a frame — split across two boxes they read as two unrelated problems,
// and next to the plain sections above them nothing said which was urgent.
func PortIsolationReport(w io.Writer, params PortIsolationReportParams) {
	lines := rules.PortIsolationLines(rules.PortIsolationLinesParams{
		Unported: params.Unported,
		Ignoring: params.Ignoring,
	})
	if len(lines) == 0 {
		return
	}
	Blank(w)
	Callout(w, domain.PortIsolationTitle, lines)
}

// PortCommandOnlyReport names the jobs whose port only travels through the
// command wtm plays. It is a note, not the alert above: they are isolated, and
// only while wtm is the one starting them.
func PortCommandOnlyReport(w io.Writer, jobs []string) {
	lines := rules.PortCommandOnlyLines(jobs)
	if len(lines) == 0 {
		return
	}
	Blank(w)
	Callout(w, domain.PortCommandOnlyTitle, lines)
}
