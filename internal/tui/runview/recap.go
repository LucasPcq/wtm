package runview

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
)

// Result is what the view leaves behind once the alternate screen is given
// back. Recap is a raw body: the command that opened the view is the one that
// frames it.
type Result struct {
	Recap    string
	Outcomes runlogs.Outcomes
	// Detached reports that the reader left before the run ended, so the rest of
	// it was reported by the surface's own sink rather than by this view. Recap
	// is empty then: the account was given line by line as it happened.
	Detached bool
}

func (m Model) result() Result {
	return Result{Recap: m.recap(), Outcomes: m.sequence.outcomes}
}

// recapBlock is one worktree's account of itself. Above a single worktree it
// reads exactly as it always has: the heading only appears when there is
// another block it could be confused with.
func (m Model) recapBlock(outcome runlogs.Outcome) []string {
	var lines []string
	if outcome.Worktree != "" {
		lines = append(lines, styles.Bold.Render(outcome.Worktree))
	}
	if len(outcome.Started) > 0 {
		lines = append(lines, fmt.Sprintf(domain.RunViewRecapRunningFmt, joinJobs(outcome.Started)))
		lines = append(lines, m.addressLines(outcome)...)
	}
	if len(outcome.Completed) > 0 {
		lines = append(lines, fmt.Sprintf(domain.RunViewRecapCompletedFmt, joinJobs(outcome.Completed)))
	}
	if len(outcome.Crashed) > 0 {
		lines = append(lines, styles.DangerText.Render(fmt.Sprintf(domain.RunViewRecapCrashedFmt, joinJobs(crashedNames(outcome.Crashed)))))
	}
	if outcome.Failed != "" {
		lines = append(lines, styles.DangerText.Render(fmt.Sprintf(domain.RunViewRecapFailedFmt, outcome.Failed)))
	}
	if len(outcome.NotStarted) > 0 {
		lines = append(lines, fmt.Sprintf(domain.RunViewRecapNotStartedFmt, joinJobs(outcome.NotStarted)))
	}
	if len(outcome.Started) == 0 {
		lines = append(lines, domain.RunViewRecapNoneRunning)
	}
	return lines
}

// addressLines say where the jobs left running answer. Only the ones that have
// an address are listed: a job with neither a published name nor a declared
// port has nothing to point at, and a blank column would read as a failure.
func (m Model) addressLines(outcome runlogs.Outcome) []string {
	started := make(map[string]bool, len(outcome.Started))
	for _, name := range outcome.Started {
		started[name] = true
	}

	var lines []string
	for _, view := range m.jobs {
		if view.WorkDir != outcome.WorkDir || !started[view.Name] {
			continue
		}
		// A runner answers for its children and for nothing of its own, so its
		// addresses are theirs: one line each, under the runner's, which is the
		// only place they are written at all. Nothing folds on a terminal the view
		// is handing back.
		if len(view.Address.Held) > 0 {
			lines = append(lines, styles.Muted.Render(fmt.Sprintf(domain.RunViewRecapAddressFmt, view.Name, rules.HeldSummaryText(len(view.Address.Held)))))
			for _, held := range rules.HeldAddressLines(view.Address.Held) {
				lines = append(lines, styles.Muted.Render(domain.RunViewRecapHeldIndent+held))
			}
			continue
		}
		address := rules.JobAddressText(view.Address)
		if address == "" {
			continue
		}
		lines = append(lines, styles.Muted.Render(fmt.Sprintf(domain.RunViewRecapAddressFmt, view.Name, address)))
	}
	return lines
}

func crashedNames(exits []domain.JobExit) []string {
	names := make([]string, 0, len(exits))
	for _, exit := range exits {
		names = append(names, exit.Job)
	}
	return names
}

func (m Model) anythingStarted() bool {
	for _, outcome := range m.sequence.outcomes {
		if len(outcome.Started) > 0 {
			return true
		}
	}
	return false
}

// recap is the last thing said about a run, on the terminal the view was
// covering: what it left running, and the two commands that act on it. A view
// that started nothing — `wtm run logs` — has nothing to conclude.
func (m Model) recap() string {
	if !m.started {
		return ""
	}

	var lines []string
	if m.profile != "" {
		lines = append(lines, fmt.Sprintf(domain.RunViewRecapProfileFmt, m.profile))
	}
	for _, outcome := range m.sequence.outcomes {
		// Each worktree's account is a block of its own. Run together they read as
		// one list of jobs, which is exactly what the heading is there to deny.
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, m.recapBlock(outcome)...)
	}

	hints := []string{styles.NextStepLine(styles.NextStepParams{Command: domain.RunStreamAttachHint, Note: domain.RunStreamAttachNote})}
	if m.anythingStarted() {
		hints = append(hints, styles.NextStepLine(styles.NextStepParams{Command: domain.RunStreamStopHint, Note: domain.RunStreamStopNote}))
	}
	body := strings.Join(append(lines, append([]string{""}, hints...)...), "\n")

	return styles.RenderRecap(styles.IntroParams{
		Width:   m.width,
		Title:   domain.RunViewRecapTitle,
		Body:    body,
		InFrame: true,
	})
}
