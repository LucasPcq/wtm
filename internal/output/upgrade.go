package output

import (
	"fmt"
	"io"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
)

func UpgradeResultJSON(w io.Writer, result domain.UpgradeResult) error {
	return encodeJSON(w, result)
}

// UpgradeReport renders the raw body of a wtm upgrade run; the command frames it.
func UpgradeReport(w io.Writer, result domain.UpgradeResult) {
	if result.UpToDate {
		Unchanged(w, fmt.Sprintf("%s %s is already the latest release", domain.AppName, result.Installed))
		return
	}

	switch result.Action {
	case domain.UpgradeActionReplaced, domain.UpgradeActionDelegated:
		Success(w, fmt.Sprintf("%s %s → %s", domain.AppName, result.Installed, result.Latest))
	case domain.UpgradeActionChecked:
		Update(w, fmt.Sprintf("%s %s → %s available", domain.AppName, result.Installed, result.Latest))
		NextStep(w, NextStepParams{Command: rules.UpgradeCommandFor(result.Method)})
	default:
		Unchanged(w, fmt.Sprintf("%s %s unchanged", domain.AppName, result.Installed))
	}
}

type UpdateNoticeParams struct {
	Current string
	Latest  string
	Method  domain.InstallMethod
}

// UpdateNotice is the passive notifier line. It is written to stderr outside any
// frame, so it carries no padding of its own.
func UpdateNotice(w io.Writer, params UpdateNoticeParams) {
	body := fmt.Sprintf("%s %s → %s · run `%s`", domain.AppName, params.Current, params.Latest, rules.UpgradeCommandFor(params.Method))
	fmt.Fprintf(w, "%s%s\n", Indent, styles.Muted.Render(body))
}
