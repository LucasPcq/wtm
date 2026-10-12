package render

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/LucasPcq/wtm/internal/kernel"
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
// it was computed on, which --expect-fingerprint hands back to apply it.
func Recap(w io.Writer, params RecapParams) {
	SectionTitle(w, fmt.Sprintf(recapTitleFmt, params.Command))
	params.Detail.Plan(w, params.Plan)
	for _, warning := range params.Warnings {
		Warning(w, WarningMessage(warning))
	}
	InfoLine(w, fingerprintLabel, params.Fingerprint)
	NextSteps(w, params.Next)
}

// View is a query's answer as label-value rows.
func View(w io.Writer, command string, view json.RawMessage) {
	Announce(w, command, rowsOf(view))
}
