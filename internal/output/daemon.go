package output

import (
	"fmt"
	"io"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func WriteDaemonStatusJSON(w io.Writer, status domain.DaemonStatus) error {
	return encodeJSON(w, status)
}

// DaemonStatusReport is a conclusion and then its detail, like every other
// command: the state used to be a field inside the readout, which left the
// reader to work the answer out of a table.
func DaemonStatusReport(w io.Writer, status domain.DaemonStatus) {
	if !status.Running {
		Unchanged(w, domain.DaemonStatusStopped)
	} else {
		Success(w, fmt.Sprintf(domain.DaemonStatusUpFmt, status.PID))
	}
	Blank(w)
	DaemonStatusFields(w, status)
}

// DaemonStatusFields is the readout alone, for a command that concluded on what
// it did rather than on what the daemon is — `run daemon restart` says it
// restarted, and a second conclusion under that says nothing new.
func DaemonStatusFields(w io.Writer, status domain.DaemonStatus) {
	items := []AnnounceItem{{Label: domain.DaemonStatusVersLabel, Value: daemonVersion(status)}}
	if status.Running {
		items = append(items, AnnounceItem{Label: domain.DaemonStatusJobsLabel, Value: fmt.Sprintf(domain.DaemonStatusJobsFmt, status.Foreground, status.Detached)})
		if status.ProxyPort > 0 {
			items = append(items, AnnounceItem{Label: domain.DaemonStatusProxyLabel, Value: fmt.Sprintf(domain.DaemonStatusProxyFmt, status.ProxyPort)})
		}
	}
	items = append(items,
		AnnounceItem{Label: domain.DaemonStatusSocketLbl, Value: status.SocketPath},
		AnnounceItem{Label: domain.DaemonStatusIndexLabel, Value: status.StatePath},
	)
	Announce(w, domain.DaemonStatusTitle, items)

	if rules.DaemonVersionDiverged(status) {
		Callout(w, domain.DaemonMismatchTitle, rules.DaemonVersionMismatchLines(rules.DaemonVersionMismatchParams{
			Client: status.Version,
			Daemon: status.DaemonVersion,
		}))
	}
}

func daemonVersion(status domain.DaemonStatus) string {
	if !status.Running || status.DaemonVersion == status.Version {
		return status.Version
	}
	return fmt.Sprintf("%s (daemon: %s)", status.Version, rules.DaemonVersionLabel(status.DaemonVersion))
}
