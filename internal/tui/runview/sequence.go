package runview

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/rules"
)

// sequence is what the view knows of a profile being started. It decides
// nothing about the run — the order, what aborts it and what is left running
// are runlogs' answers, carried here as they are emitted.
type sequence struct {
	active bool
	// keys are the jobs being started right now — several since a run covers
	// several worktrees, each stepping through its own sequence. What is in here
	// is what the run alone is writing into, and what no pane may be released
	// from under. job is the last one's name, for a header with no room for a path.
	keys map[jobKey]bool
	job  string
	step int
	// pending counts the worktrees that have not concluded yet. One of them
	// ending says nothing about the others, so the sequence is over only when
	// every one has reported.
	pending int
	steps   int
	states  map[jobKey]domain.JobStep
	// ports is what each job bound as it started, kept for the pane title: the
	// run is the only moment the daemon reports them.
	ports map[jobKey]map[string]int
	// urls is where each job answers, kept for the same reason as ports: the run
	// is the only moment it is reported.
	urls map[jobKey]string
	// devOrigins are the config lines the started jobs are missing, collected as
	// they are reported so the recap can name them all at once.
	devOrigins []domain.DevOriginFix
	// notices are the facts the run reported about itself rather than about one
	// of its jobs — a proxy that could not bind, so far.
	notices []string
	// reasons are what the daemon answered for the job that ended each worktree's
	// sequence, keyed by worktree: N of them abort independently, and one
	// reason for the lot would name the last failure for every one.
	reasons map[string]string
	// outcomes are what the sequences concluded, one per worktree in the order
	// they reported. A run over several worktrees ends several times, and the
	// last to finish must not be the only one with an account of itself.
	outcomes runlogs.Outcomes
}

// record keeps what a worktree concluded, replacing the account it had given
// before. A worktree that reported nothing does not erase one that did.
func (s *sequence) record(outcome runlogs.Outcome) {
	if !outcome.Recorded() {
		return
	}
	for index := range s.outcomes {
		if s.outcomes[index].WorkDir == outcome.WorkDir {
			s.outcomes[index] = outcome
			return
		}
	}
	s.outcomes = append(s.outcomes, outcome)
}

// hold and release bracket the moment a job's only copy of its output is the
// pane the run is writing into: nothing replays that yet, so the pane must
// survive the cursor moving away from it.
func (s *sequence) hold(key jobKey, step domain.JobStep) {
	s.keys[key] = true
	s.states[key] = step
}

func (s *sequence) release(key jobKey, step domain.JobStep) {
	delete(s.keys, key)
	s.states[key] = step
	s.job = ""
}

// aborted is the worktrees that stopped short, in the order they reported.
func (s sequence) aborted() runlogs.Outcomes {
	var aborted runlogs.Outcomes
	for _, outcome := range s.outcomes {
		if outcome.Aborted() {
			aborted = append(aborted, outcome)
		}
	}
	return aborted
}

// probes is every worktree's port verdicts, gathered: the check is per job, and
// a job's name alone does not say which worktree silenced its port.
func (s sequence) probes() []domain.PortProbe {
	var probes []domain.PortProbe
	for _, outcome := range s.outcomes {
		probes = append(probes, outcome.Probes...)
	}
	return probes
}

type eventMsg struct{ event runlogs.Event }

// openFailedMsg reports a browser that never opened, so the key press does not
// read as a no-op.
type openFailedMsg struct{ err error }

type runFinishedMsg struct {
	outcomes runlogs.Outcomes
	err      error
}

// sink carries a run's events to the model. A job's output never goes through
// the channel: it is written into the pane on this goroutine as it arrives, and
// the model's clock is what puts it on screen.
type sink struct {
	panes *paneStore
	msgs  chan<- tea.Msg
	done  <-chan struct{}
}

func (s sink) Emit(event runlogs.Event) {
	if event.Phase == runlogs.PhaseOutput {
		s.panes.write(writeChunkParams{
			Key:          eventKey(event),
			Source:       sourceSequence,
			Chunk:        event.Chunk,
			NormalizeEOL: rules.RunsOnPipe(event.Kind),
		})
		return
	}
	select {
	case s.msgs <- eventMsg{event: event}:
	case <-s.done:
	}
}

func (m Model) applyEvent(msg eventMsg) (Model, tea.Cmd) {
	event := msg.event
	noticeLines := len(m.report())
	m.sequence.steps = max(event.Steps, m.sequence.steps)

	switch event.Phase {
	case runlogs.PhaseStarting:
		m.sequence.active = true
		m.sequence.job, m.sequence.step = event.Job, event.Step
		m.sequence.hold(eventKey(event), domain.JobStepStarting)
	case runlogs.PhaseStarted:
		m.sequence.release(eventKey(event), domain.JobStepStarted)
		m.sequence.remember(event)
	case runlogs.PhaseDone:
		m.sequence.release(eventKey(event), domain.JobStepDone)
		m.sequence.remember(event)
	case runlogs.PhaseFailed:
		m.sequence.release(eventKey(event), domain.JobStepFailed)
		m.sequence.reasons[event.WorkDir] = event.Reason
	case runlogs.PhaseNotice:
		m.sequence.notices = append(m.sequence.notices, event.Notice)
	case runlogs.PhaseAborted, runlogs.PhaseReady:
		// One worktree ending says nothing about the others: the sequence is over
		// when every one of them has reported.
		m.sequence.job = ""
		m.sequence.record(event.Outcome)
		m.sequence.pending = max(m.sequence.pending-1, 0)
		m.sequence.active = m.sequence.pending > 0
	}

	model, cmd := m.followSequence(event)
	model, resized := model.resyncSize(noticeLines)

	// The clock is the only thing that draws a pane the run writes into: a
	// PhaseOutput never reaches the model, and a tick that landed before the
	// first phase found nothing scheduled and stopped.
	var tick tea.Cmd
	if model.sequence.active {
		model, tick = model.startTicking()
	}
	return model, tea.Batch(cmd, resized, tick, model.refreshCmd(), model.listenCmd())
}

// followSequence puts the job the sequence is acting on in front of the reader,
// until the reader takes the cursor themselves.
func (m Model) followSequence(event runlogs.Event) (Model, tea.Cmd) {
	if !m.following || event.Job == "" || eventKey(event) == m.selected {
		return m.fillSelectedPane()
	}
	return m.setSelection(eventKey(event))
}

// applyRunFinished records what the run ended with. The error is the run's own,
// not a job's: a sequence that was detached from ended on the context, which is
// the view being left rather than anything to report.
func (m Model) applyRunFinished(msg runFinishedMsg) (Model, tea.Cmd) {
	noticeLines := len(m.report())
	m.sequence.active = false
	m.runDone = true
	for _, outcome := range msg.outcomes {
		m.sequence.record(outcome)
	}
	if msg.err != nil && m.runCtx.Err() == nil {
		m.err = msg.err
	}
	model, resized := m.resyncSize(noticeLines)
	return model, tea.Batch(resized, model.refreshCmd())
}

// report is the notice area: what the run has to say once it has stopped
// moving. An abort outranks a silent port — a profile that never finished is
// the bigger news, and both at once would bury it. It is built from the
// outcomes rather than from a running tally, which is what keeps it true after
// a detach, and every worktree that aborted is named: they abort
// independently, so reporting one would hide the rest.
func (m Model) report() []string {
	if m.dismissed {
		return nil
	}
	aborted := m.sequence.aborted()
	if len(aborted) == 0 {
		return m.probeReport()
	}

	lines := []string{domain.RunViewAbortTitle}
	for index, outcome := range aborted {
		if index > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, m.abortLines(outcome)...)
	}
	return append(lines, domain.RunViewAbortDismiss)
}

func (m Model) abortLines(outcome runlogs.Outcome) []string {
	// The worktree qualifies the job, not the whole line: the reason trails it and
	// can run long, and a name at the far end of it is a name nobody reads.
	lines := []string{fmt.Sprintf(domain.RunViewAbortFailedFmt,
		outcome.FailedStep, outcome.Steps,
		m.qualify(outcome.Failed, outcome.Worktree),
		m.sequence.reasons[outcome.WorkDir],
	)}
	if len(outcome.Started) > 0 {
		lines = append(lines, fmt.Sprintf(domain.RunViewAbortRunningFmt, joinJobs(outcome.Started)))
	}
	if len(outcome.NotStarted) > 0 {
		lines = append(lines, fmt.Sprintf(domain.RunViewAbortNotStartedFmt, joinJobs(outcome.NotStarted)))
	}
	return lines
}

// probeReport names the declared ports nothing answered on, once the sequence
// is over. A run still starting has nothing to conclude yet.
func (m Model) probeReport() []string {
	if m.sequence.active || !m.sequence.outcomes.Recorded() {
		return m.addressingReport()
	}
	lines := rules.PortProbeLines(m.sequence.probes())
	if len(lines) == 0 {
		return m.devOriginsReport()
	}
	return append(append([]string{domain.PortProbeTitle}, lines...), domain.RunViewAbortDismiss)
}

// devOriginsReport names the Next projects that will refuse their own name. It
// yields to the port report above for the same reason that one yields to an
// abort: one notice area, the more urgent finding first.
func (m Model) devOriginsReport() []string {
	if len(m.sequence.devOrigins) == 0 {
		return m.noticeReport()
	}
	lines := []string{domain.DevOriginsTitle}
	for _, fix := range m.sequence.devOrigins {
		lines = append(lines, fix.Line)
	}
	return append(lines, domain.RunViewAbortDismiss)
}

// noticeReport is the last thing the notice area falls back to: what the run
// has to say about itself once no job has anything more urgent.
func (m Model) noticeReport() []string {
	if len(m.sequence.notices) == 0 {
		return m.addressingReport()
	}
	lines := append([]string{domain.ProxyUnavailableTitle}, m.sequence.notices...)
	return append(lines, domain.RunViewAbortDismiss)
}

// addressingReport is the tail of the chain: a fact about the addresses the
// view is publishing, which no run has to happen for. It is the only band
// `run logs` can show, since nothing there ever starts.
func (m Model) addressingReport() []string {
	if len(m.warnings) == 0 {
		return nil
	}
	lines := append([]string{domain.AddressingDriftTitle}, m.warnings...)
	return append(lines, domain.RunViewAbortDismiss)
}

func joinJobs(jobs []string) string {
	return strings.Join(jobs, domain.RunViewRecapListSep)
}

func (s *sequence) remember(event runlogs.Event) {
	if len(event.Ports) > 0 {
		if s.ports == nil {
			s.ports = map[jobKey]map[string]int{}
		}
		s.ports[eventKey(event)] = event.Ports
	}
	s.devOrigins = append(s.devOrigins, event.DevOrigins...)

	if event.URL == "" {
		return
	}
	if s.urls == nil {
		s.urls = map[jobKey]string{}
	}
	s.urls[eventKey(event)] = event.URL
}
