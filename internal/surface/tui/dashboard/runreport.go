package dashboard

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/rules"
)

// runEventLines is what the panel says about the phases a start reports beside
// its steps, in the CLI's words (output.RunPrinter): a job that died after its
// ✓, the ports nobody bound, and what an abort left standing. The abort's own
// headline is the run's conclusion, written once the flow returns.
func runEventLines(event runlogs.Event) []string {
	switch event.Phase {
	case runlogs.PhaseCrashed:
		reason := event.Reason
		if event.ExitCode != nil {
			reason = fmt.Sprintf(domain.RunStreamCrashedCodeFmt, reason, *event.ExitCode)
		}
		return []string{fmt.Sprintf(domain.RunDetachedCrashedFmt, event.Job, reason)}
	case runlogs.PhaseWarning:
		return []string{fmt.Sprintf(domain.RunDetachedWarningFmt, event.Notice)}
	case runlogs.PhaseProbed:
		probes := rules.PortProbeLines(event.Probes)
		if len(probes) == 0 {
			return nil
		}
		lines := []string{domain.RunDetachedProbeTitle}
		for _, probe := range probes {
			lines = append(lines, fmt.Sprintf(domain.RunDetachedDetailFmt, probe))
		}
		return lines
	case runlogs.PhaseAborted:
		var lines []string
		if started := event.Outcome.Started; len(started) > 0 {
			lines = append(lines, fmt.Sprintf(domain.RunDetachedLeftRunning, strings.Join(started, domain.RunViewRecapListSep)))
		}
		if missed := event.Outcome.NotStarted; len(missed) > 0 {
			lines = append(lines, fmt.Sprintf(domain.RunDetachedNotStarted, strings.Join(missed, domain.RunViewRecapListSep)))
		}
		return lines
	}
	return nil
}

type runConclusionParams struct {
	Kind     string
	Outcomes runlogs.Outcomes
}

// runConclusion is the line each worktree's sequence ends on. A sequence that
// never reached a job has nothing to conclude: its refusal was already said.
func runConclusion(params runConclusionParams) []string {
	var lines []string
	for _, outcome := range params.Outcomes {
		if !outcome.Recorded() {
			continue
		}
		name := outcome.Worktree
		if name == "" {
			name = outcome.WorkDir
		}
		switch {
		case outcome.Aborted():
			lines = append(lines, fmt.Sprintf(domain.RunDetachedAbortedFmt, params.Kind, name, outcome.FailedStep, outcome.Steps, outcome.Failed))
		case len(outcome.Crashed) > 0:
			lines = append(lines, fmt.Sprintf(domain.RunDetachedCrashedEndFmt, params.Kind, name, crashedNames(outcome.Crashed)))
		default:
			lines = append(lines, fmt.Sprintf(domain.RunDetachedConcludedFmt, params.Kind, name))
		}
	}
	return lines
}

func crashedNames(exits []domain.JobExit) string {
	names := make([]string, 0, len(exits))
	for _, exit := range exits {
		names = append(names, exit.Job)
	}
	return strings.Join(names, domain.RunViewRecapListSep)
}
