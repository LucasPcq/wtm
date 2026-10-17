package output

import (
	"io"

	"github.com/LucasPcq/wtm/next/internal/app"
)

type DryRunParams struct {
	Outcome app.CreateOutcome
	Apply   string
}

func DryRun(w io.Writer, params DryRunParams) {
	sectionTitle(w, commandName+" · dry run, nothing applied")
	warnings := make([]string, 0, len(params.Outcome.Warnings))
	for _, warned := range params.Outcome.Warnings {
		warnings = append(warnings, warned.Message)
	}
	rows := planRowsOf(params.Outcome.Plan)
	writePlan(w, renderPlan(planParams{Rows: rows, Warnings: warnings, Width: max(terminalWidth(w)-len(indent), 0)}))
	if len(warnings) > 0 {
		blank(w)
	}
	infoLine(w, "fingerprint", params.Outcome.Fingerprint)
	nextSteps(w, []NextStep{{Command: params.Apply, Note: "apply this plan"}})
}
