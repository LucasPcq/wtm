package dashboard

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/run/seam"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
)

// detachedWatcher drives a start sequence without asking for the terminal. The
// dashboard has a list one wants to come back over, so a start there gives the
// surface back and reports; watching a run is what the logs view is for.
type detachedWatcher struct {
	ctx  context.Context
	send func(tea.Msg)
	id   int
}

func (w detachedWatcher) Sequence(params seam.SequenceParams) (runlogs.Outcomes, error) {
	if params.Start == nil {
		return nil, nil
	}
	return params.Start(w.ctx, detachedSink{
		send: w.send,
		id:   w.id,
		// N sequences interleave in one panel, and two jobs called `web` are
		// otherwise the same line twice. Above a single worktree naming it would
		// only repeat what the user has just ticked.
		multi: len(params.Worktrees) > 1,
	})
}

// detachedSink turns the sequence's events into output-panel lines and into the
// stage the held row shows beside its spinner. PhaseOutput says nothing: a job's
// raw bytes belong to the logs view.
type detachedSink struct {
	send  func(tea.Msg)
	id    int
	multi bool
}

func (s detachedSink) Emit(event runlogs.Event) {
	switch event.Phase {
	case runlogs.PhaseStarting:
		stage := fmt.Sprintf(domain.RunDetachedStartingFmt, event.Job, event.Step, event.Steps)
		// The stage is posted bare: it sits on the row of the worktree it came
		// from, which names it already.
		s.send(opStageMsg{id: s.id, target: s.target(event), stage: stage})
		s.emit(stage, event)
	case runlogs.PhaseStarted:
		if event.AlreadyRunning {
			already := domain.RunDetachedAlreadyFmt
			if event.Joined {
				already = domain.RunDetachedAlreadyJoinedFmt
			}
			s.emit(fmt.Sprintf(already, event.Job), event)
			return
		}
		started := domain.RunDetachedStartedFmt
		if event.Joined {
			started = domain.RunDetachedJoinedFmt
		}
		s.emit(fmt.Sprintf(started, event.Job), event)
		if event.Namespace != "" {
			s.emit(fmt.Sprintf(domain.RunDetachedNamespaceFmt, event.Job, event.Namespace), event)
		}
		if event.URL != "" {
			s.emit(fmt.Sprintf(domain.RunDetachedAddressFmt, event.Job, event.URL), event)
		}
	// A task that ran to its end is done, not up: PhaseDone concludes a task,
	// where PhaseStarted announces a service.
	case runlogs.PhaseDone:
		s.emit(fmt.Sprintf(domain.RunDetachedDoneFmt, event.Job), event)
	case runlogs.PhaseFailed:
		s.emit(fmt.Sprintf(domain.RunDetachedFailedFmt, event.Job, event.Reason), event)
	case runlogs.PhaseNotice:
		s.emit(event.Notice, event)
	default:
		for _, line := range runEventLines(event) {
			s.emit(line, event)
		}
	}
}

// emit posts a line the panel can attribute. The rule is the CLI's
// (output.RunPrinter.qualify): the worktree is named above several of them, and
// left out above one.
func (s detachedSink) emit(text string, event runlogs.Event) {
	if !s.multi || s.target(event) == "" {
		s.line(text)
		return
	}
	s.line(fmt.Sprintf(domain.RunStreamWorktreeFmt, text, s.target(event)))
}

// target names the worktree an event came from: its branch, and its path for a
// worktree git cannot name.
func (s detachedSink) target(event runlogs.Event) string {
	if event.Worktree != "" {
		return event.Worktree
	}
	return event.WorkDir
}

func (s detachedSink) line(text string) {
	if text == "" {
		return
	}
	s.send(OutputLineMsg{Text: text})
}
