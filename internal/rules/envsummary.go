package rules

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
)

// EnvSummary is the trailing verdict of `wtm env`: what to say, and whether it
// reads as an accomplishment or as a plain note.
type EnvSummary struct {
	Text string
	Done bool
}

// EnvOutcomeSummary tallies everything the run actually wrote. The port pass is
// counted alongside the files because it writes to the same .env: a run that
// shifted a port and changed nothing else has written changes, and saying "no
// changes written" there is not a wording problem, it is a false report.
// EnvUnresolvableFiles names the configured files that exist nowhere.
func EnvUnresolvableFiles(result domain.EnvSyncResult) []string {
	var names []string
	for _, file := range result.Files {
		if file.Unresolvable {
			names = append(names, file.Target)
		}
	}
	return names
}

func EnvOutcomeSummary(result domain.EnvSyncResult) EnvSummary {
	// A file the repository does not have anywhere is never "no drift": nothing
	// is missing from it because nothing can ever be in it, and the fix is in
	// config.toml rather than in any worktree.
	if unresolvable := EnvUnresolvableFiles(result); len(unresolvable) > 0 {
		return EnvSummary{Text: fmt.Sprintf(domain.EnvUnresolvableSummaryFmt, len(unresolvable))}
	}
	if result.Check {
		if EnvHasDrift(result) {
			return EnvSummary{Text: domain.EnvCheckDriftMessage}
		}
		return EnvSummary{Text: domain.EnvCheckCleanMessage, Done: true}
	}

	// The owned keys are counted with the ports: both are values wtm writes into
	// the same .env, and a run whose only work was moving a DATABASE_URL onto
	// this worktree's slice has written changes.
	files, ports := EnvAppliedFiles(result), 0
	if result.Ports.Applied {
		ports = len(EnvPortRewrites(result.Ports)) + len(OwnedEnvRewrites(result.Ports))
	}
	switch {
	case files == 0 && ports == 0:
		return EnvSummary{Text: domain.EnvNothingWrittenMessage}
	case ports == 0:
		return EnvSummary{Text: fmt.Sprintf(domain.EnvReconciledFmt, files), Done: true}
	case files == 0:
		return EnvSummary{Text: fmt.Sprintf(domain.EnvPortsShiftedFmt, ports), Done: true}
	default:
		return EnvSummary{Text: fmt.Sprintf(domain.EnvReconciledAndShiftedFmt, files, ports), Done: true}
	}
}
