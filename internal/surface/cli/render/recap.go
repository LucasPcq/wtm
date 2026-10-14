package render

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/kernel/text"
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
	Catalog     text.Catalog
}

func Recap(w io.Writer, params RecapParams) {
	SectionTitle(w, fmt.Sprintf(recapTitleFmt, params.Command))
	warnings := make([]string, 0, len(params.Warnings))
	for _, warning := range params.Warnings {
		warnings = append(warnings, params.Catalog.Message(warning.Code, warning.Params))
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
