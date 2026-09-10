package dashboard

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
)

// detachedRun is where a run reports once the reader has closed the view it was
// started from. The sequence belongs to this process — the daemon cannot take
// it over — so leaving the view stops the watching, not the run, and the jobs it
// had not reached yet still have to be started and still have to be accounted
// for. The output panel is where the dashboard already puts what a flow says.
type detachedRun struct {
	send func(tea.Msg)
}

func (d detachedRun) Emit(event runlogs.Event) {
	// A job's raw bytes are not a line and the panel holds lines. They are in
	// the job's log either way, which is what the LOGS tab reads.
	if event.Phase == runlogs.PhaseOutput {
		return
	}
	if line := detachedLine(event); line != "" {
		d.send(OutputLineMsg{Text: line})
	}
}

func detachedLine(event runlogs.Event) string {
	switch event.Phase {
	case runlogs.PhaseStarting:
		return fmt.Sprintf(domain.RunStreamStepFmt, event.Step, event.Steps, event.Job)
	case runlogs.PhaseStarted:
		if event.AlreadyRunning {
			return fmt.Sprintf(domain.RunStreamAlreadyFmt, event.Job)
		}
		return fmt.Sprintf(domain.RunStreamStartedFmt, event.Job)
	case runlogs.PhaseDone:
		return fmt.Sprintf(domain.RunStreamDoneFmt, event.Job)
	case runlogs.PhaseFailed:
		return event.Reason
	case runlogs.PhaseCrashed:
		return fmt.Sprintf(domain.RunStreamCrashedFmt, event.Job, event.Reason)
	case runlogs.PhaseNotice:
		return event.Notice
	}
	return ""
}
