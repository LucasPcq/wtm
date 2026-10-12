package render

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/styles"
)

const (
	recapTitleFmt    = "%s · dry run, nothing applied"
	fingerprintLabel = "fingerprint"
)

type RecapParams struct {
	Command     string
	Plan        json.RawMessage
	Warnings    []kernel.Warning
	Fingerprint string
	Next        []NextStepParams
	Detail      Detail
}

// Recap is what a dry run answers: the plan, then the fingerprint of the facts
// it was computed on, which --expect-fingerprint hands back to apply it. A plan
// that does nothing offers nothing to apply.
func Recap(w io.Writer, params RecapParams) {
	SectionTitle(w, fmt.Sprintf(recapTitleFmt, params.Command))
	warnings := make([]string, 0, len(params.Warnings))
	for _, warning := range params.Warnings {
		warnings = append(warnings, WarningMessage(warning))
	}
	rows := params.Detail.Plan(params.Plan)
	plan := styles.RenderPlan(styles.PlanParams{Rows: rows, Warnings: warnings, Width: max(TerminalWidthOf(w)-len(Indent), 0)})
	for line := range strings.Lines(plan) {
		indented(w, strings.TrimSuffix(line, "\n"))
	}
	if len(warnings) > 0 {
		fmt.Fprintln(w)
	}
	InfoLine(w, fingerprintLabel, params.Fingerprint)
	if len(rows) > 0 {
		NextSteps(w, params.Next)
	}
}

func indented(w io.Writer, line string) {
	if line == "" {
		fmt.Fprintln(w)
		return
	}
	fmt.Fprintln(w, Indent+line)
}

// View is a query's answer as label-value rows.
func View(w io.Writer, command string, view json.RawMessage) {
	Announce(w, command, rowsOf(view))
}
