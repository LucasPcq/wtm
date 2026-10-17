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
	warnings := messages(params.Outcome.Warnings)
	drawPlan(w, planParams{Rows: planRowsOf(params.Outcome.Plan), Warnings: warnings})
	if len(warnings) > 0 {
		blank(w)
	}
	infoLine(w, info{Label: "fingerprint", Value: params.Outcome.Fingerprint})
	nextSteps(w, []NextStep{{Command: params.Apply, Note: "apply this plan"}})
}
