package output

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

func WriteProxyStatusJSON(w io.Writer, status domain.ProxyStatus) error {
	return encodeJSON(w, status)
}

// ProxyStatusReport is a conclusion and then its detail. Whether the
// redirection is installed is the question the command was asked, so it is the
// line above the readout rather than one row inside it.
func ProxyStatusReport(w io.Writer, status domain.ProxyStatus) {
	switch {
	case !status.Supported:
		Warning(w, domain.ProxyStatusUnsupported)
	case !status.Installed:
		Unchanged(w, domain.ProxyStatusNotInstalled)
	default:
		Success(w, fmt.Sprintf(domain.ProxyStatusInstalledFmt, status.Mechanism, status.BindPort))
	}
	Blank(w)

	items := []AnnounceItem{
		{Label: domain.ProxyStatusBindLabel, Value: strconv.Itoa(status.BindPort)},
		{Label: domain.ProxyStatusPublicLabel, Value: strconv.Itoa(status.PublicPort)},
	}
	if status.ConfigPath != "" {
		items = append(items, AnnounceItem{Label: domain.ProxyStatusConfigLabel, Value: status.ConfigPath})
	}
	if status.ExampleURL != "" {
		items = append(items, AnnounceItem{Label: domain.ProxyStatusExampleLabel, Value: status.ExampleURL})
	}
	Announce(w, domain.ProxyStatusTitle, items)

	if status.Diverged {
		Callout(w, domain.ProxyDivergedTitle, []string{domain.ProxyDivergedLine, domain.ProxyDivergedFix})
	}
}

type ProxyPlanReportParams struct {
	Files  []domain.ProxyPlannedFile
	Script string
	// Full prints each file's contents rather than what changes in it.
	Full bool
	// Reversible adds the two lines only an install has to offer.
	Reversible bool
}

// ProxyPlanReport shows what a privileged write touches before it is asked for.
func ProxyPlanReport(w io.Writer, params ProxyPlanReportParams) {
	SectionTitle(w, fmt.Sprintf(domain.ProxyInstallRecapTitleFmt, len(params.Files)))
	Blank(w)

	if params.Full {
		for _, file := range params.Files {
			Section(w, file.Path, strings.Split(strings.TrimRight(file.Content, "\n"), "\n"))
		}
	}
	if !params.Full {
		lines := make([]string, 0, len(params.Files))
		for _, file := range params.Files {
			lines = append(lines, fmt.Sprintf(domain.ProxyPlanFileFmt, file.Path, file.Change))
		}
		for _, line := range lines {
			Message(w, line)
		}
		Blank(w)
	}

	if params.Script != "" {
		Section(w, domain.ProxyInstallRecapScript, strings.Split(params.Script, "\n"))
		Blank(w)
	}
	if params.Reversible {
		NextStep(w, NextStepParams{Command: domain.ProxyInstallRecapReverse, Note: domain.ProxyInstallRecapReverseNote})
		if !params.Full {
			NextStep(w, NextStepParams{Command: domain.ProxyInstallRecapFull, Note: domain.ProxyInstallRecapFullNote})
		}
	}
}
