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
	sectionTitle(w, commandName+dot+"dry run, nothing applied")
	warnings := make([]string, 0, len(params.Outcome.Warnings))
	for _, warned := range params.Outcome.Warnings {
		warnings = append(warnings, warned.Message)
	}
	drawPlan(w, planParams{Rows: planRowsOf(params.Outcome.Plan), Warnings: warnings})
	if len(warnings) > 0 {
		blank(w)
	}
	infoLine(w, "fingerprint", params.Outcome.Fingerprint)
	nextSteps(w, []NextStep{{Command: params.Apply, Note: "apply this plan"}})
}
