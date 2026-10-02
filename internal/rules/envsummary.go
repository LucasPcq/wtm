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
			return EnvSummary{Text: fmt.Sprintf(domain.EnvCheckDriftMessage, result.Branch), Verdict: domain.EnvVerdictAttention}
		}
		return EnvSummary{Text: domain.EnvCheckCleanMessage}
	}

	written := envWrittenSummary(result)
	switched := envSwitchSummary(result)
	switch {
	case switched == "":
		return written
	case written.Verdict == domain.EnvVerdictNeutral:
		return EnvSummary{Text: switched, Verdict: domain.EnvVerdictDone}
	default:
		return EnvSummary{Text: switched + " " + written.Text, Verdict: domain.EnvVerdictDone}
	}
}

// envSwitchSummary says what a run did to the worktree's isolation, before
// anything it wrote: a switch that changed no file still changed how its jobs
// run, and "no changes written" under it would be false.
func envSwitchSummary(result domain.EnvSyncResult) string {
	restored := len(result.Restored)
	switch {
	case result.IsolationChanged && restored > 0:
		return fmt.Sprintf(domain.EnvSwitchedRestoredFmt, result.Isolation, restored)
	case result.IsolationChanged:
		return fmt.Sprintf(domain.EnvSwitchedFmt, result.Isolation)
	case restored > 0:
		return fmt.Sprintf(domain.EnvRestoredFmt, restored)
	default:
		return ""
	}
}

func envWrittenSummary(result domain.EnvSyncResult) EnvSummary {
	// The owned keys are counted with the ports: both are values wtm writes into
	// the same .env, and a run whose only work was moving a DATABASE_URL onto
	// this worktree's namespace has written changes.
	files, ports := EnvAppliedFiles(result), 0
	if result.Ports.Applied {
		ports = len(EnvPortRewrites(result.Ports)) + len(OwnedEnvRewrites(result.Ports))
	}
	switch {
	case files == 0 && ports == 0:
		return EnvSummary{Text: domain.EnvNothingWrittenMessage}
	case ports == 0:
		return EnvSummary{Text: fmt.Sprintf(domain.EnvReconciledFmt, files), Verdict: domain.EnvVerdictDone}
	case files == 0:
		return EnvSummary{Text: fmt.Sprintf(domain.EnvPortsShiftedFmt, ports) + composeProjectWritten(result.Ports), Verdict: domain.EnvVerdictDone}
	default:
		return EnvSummary{Text: fmt.Sprintf(domain.EnvReconciledAndShiftedFmt, files, ports) + composeProjectWritten(result.Ports), Verdict: domain.EnvVerdictDone}
	}
}

// composeProjectWritten names the compose project a run moved the worktree to:
// its volumes are the ones a `docker compose` typed there now reaches, which is
// not a detail to leave inside a count.
func composeProjectWritten(plan domain.EnvPortPlan) string {
	for _, entry := range OwnedEnvRewrites(plan) {
		if entry.Key == domain.EnvComposeProjectName {
			return fmt.Sprintf(domain.EnvComposeProjectWrittenFmt, domain.EnvComposeProjectName, entry.Value)
		}
	}
	return ""
}
