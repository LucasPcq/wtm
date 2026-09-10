package rules

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
)

// EnvSummary is the trailing verdict of `wtm env`: what to say, and which
// register to say it in.
type EnvSummary struct {
	Text    string
	Verdict domain.EnvVerdict
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
		return EnvSummary{Text: fmt.Sprintf(domain.EnvUnresolvableSummaryFmt, len(unresolvable)), Verdict: domain.EnvVerdictAttention}
	}
	if result.Check {
		if EnvHasDrift(result) {
			// A read-only run that found drift did not leave the worktree in the
			// state it wants: there is something to do, and it is the reader's.
			return EnvSummary{Text: domain.EnvCheckDriftMessage, Verdict: domain.EnvVerdictAttention}
		}
		return EnvSummary{Text: domain.EnvCheckCleanMessage, Verdict: domain.EnvVerdictDone}
	}

	files, ports := EnvAppliedFiles(result), 0
	if result.Ports.Applied {
		ports = len(EnvPortRewrites(result.Ports))
	}
	switch {
	case files == 0 && ports == 0:
		return EnvSummary{Text: domain.EnvNothingWrittenMessage}
	case ports == 0:
		return EnvSummary{Text: fmt.Sprintf(domain.EnvReconciledFmt, files), Verdict: domain.EnvVerdictDone}
	case files == 0:
		return EnvSummary{Text: fmt.Sprintf(domain.EnvPortsShiftedFmt, ports), Verdict: domain.EnvVerdictDone}
	default:
		return EnvSummary{Text: fmt.Sprintf(domain.EnvReconciledAndShiftedFmt, files, ports), Verdict: domain.EnvVerdictDone}
	}
}
