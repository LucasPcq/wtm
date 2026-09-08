package output

import (
	"fmt"
	"io"
	"strconv"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
)

// EnvPortsReport prints the .env values a worktree's offset moves, and the links
// wtm declined to act on. It is a section like the file blocks it follows, not a
// boxed callout: the two describe the same run on the same files, and two visual
// languages in one report make it read as two unrelated reports.
//
// It emits a raw body with no surrounding blank lines; the caller's frame owns
// the padding. A plan with nothing to say prints nothing.
func EnvPortsReport(w io.Writer, plan domain.EnvPortPlan, check bool) {
	// The section is indented twice like every Message it prints, and that is
	// room the table does not have.
	rows := rules.EnvPortTableLines(rules.EnvPortTableParams{
		Plan:  plan,
		Width: reportWidth(),
	})
	anomalies := rules.EnvPortAnomalyLines(plan)
	notices := rules.EnvPortNotices(plan)
	if len(rows) == 0 && len(anomalies) == 0 && len(notices) == 0 {
		return
	}

	Blank(w)

	// A declined pass collapses to one line. The table was the proposal; once the
	// answer is no, re-printing it in the result report is a list of things that
	// did not happen. A --check run is a preview, so it keeps its table.
	declined := len(rows) > 0 && !plan.Applied && !check
	if declined {
		Unchanged(w, fmt.Sprintf(domain.EnvPortsLeftAloneFmt, len(rules.EnvPortRewrites(plan))))
		printEnvPortAnomalies(w, anomalies)
		printEnvPortNotices(w, notices, false)
		return
	}

	// The title names the table, so it comes with one. A pass that only has a
	// notice to give — every value already settled — would otherwise print a
	// heading over nothing.
	if len(rows) > 0 {
		SectionTitle(w, fmt.Sprintf(domain.EnvFileHeaderFmt,
			domain.EnvPortsTitle,
			styles.Muted.Render(domain.EnvPortOffsetPrefix+strconv.Itoa(plan.Offset))))
	}

	for _, line := range rows {
		Message(w, line)
	}
	if len(rows) > 0 && len(anomalies) > 0 {
		Blank(w)
	}
	printEnvPortAnomalies(w, anomalies)
	printEnvPortNotices(w, notices, len(rows) == 0 && len(anomalies) == 0)
}

// reportWidth is what a table printed as a section has left, zero when there is
// no terminal to measure.
func reportWidth() int {
	cols := TerminalWidth()
	if cols <= 0 {
		return 0
	}
	return cols - len([]rune(Indent))
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

// EnvPortLinksReport names the links a `run init` just wrote. It is deliberately
// not a count: a reader told "1 link written" has to open run.toml to learn what
// they agreed to.
func EnvPortLinksReport(w io.Writer, links []domain.EnvPortLink, bases map[domain.PortRef]int) {
	if len(links) == 0 {
		return
	}
	Blank(w)
	Section(w, domain.EnvPortsLinkedTitle, rules.EnvPortLinkLines(links, bases))
}

// PortKeysReport names the ports a run just wrote into the project's env files.
func PortKeysReport(w io.Writer, writes []domain.PortKeyWrite) {
	if len(writes) == 0 {
		return
	}
	Blank(w)
	Section(w, domain.PortKeysTitle, rules.PortKeyLines(writes))
}
