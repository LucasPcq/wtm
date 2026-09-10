package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
)

type RunPrinterParams struct {
	// Out and Err are the command's own streams, unbarred. The printer bars the
	// lines it composes itself and writes a job's bytes through untouched — see
	// Worktrees.
	Out io.Writer
	Err io.Writer
	// Profile names the run being reported. Empty prints no heading: a run.toml
	// with no profile has nothing to name.
	Profile string
	// Hyperlinks turns a job's URL into an OSC-8 link. Off for a pipe, a JSON
	// run, or anything that would only show the escape sequence.
	Hyperlinks bool
	// Worktrees are the worktrees the run covers. More than one makes every line
	// this printer composes name where it came from: N sequences interleave on one
	// stream, and two jobs called `web` are otherwise the same line twice. A job's
	// own bytes go through untouched — an escape sequence cut by a prefix is worse
	// than an unattributed line.
	Worktrees []string
}

// RunPrinter renders a profile's start sequence as lines on the terminal the
// command was launched from. It writes a raw body: the command's frame owns the
// outer padding, and the blank line between two steps is this printer's.
type RunPrinter struct {
	out io.Writer
	err io.Writer
	// raw is where a job's own bytes go: the same stream as out, without the
	// accent bar. A bar re-marks a row after every carriage return, which is
	// exactly what a progress bar redrawing itself produces.
	raw io.Writer
	// midLine says the last chunk left the cursor somewhere other than column
	// zero, so the next composed line has to break first or it would continue the
	// job's unfinished row.
	midLine    bool
	profile    string
	hyperlinks bool
	multi      bool
	worktrees  int
	printed    bool
	readied    bool
}

func NewRunPrinter(params RunPrinterParams) *RunPrinter {
	return &RunPrinter{
		out:        Barred(params.Out),
		err:        Barred(params.Err),
		raw:        params.Out,
		profile:    params.Profile,
		hyperlinks: params.Hyperlinks,
		multi:      len(params.Worktrees) > 1,
		worktrees:  len(params.Worktrees),
	}
}

func (p *RunPrinter) Emit(event runlogs.Event) {
	if event.Phase != runlogs.PhaseOutput {
		p.breakJobLine()
	}
	switch event.Phase {
	case runlogs.PhaseStarting:
		if p.printed {
			Blank(p.out)
		}
		if !p.printed && p.profile != "" {
			SectionTitle(p.out, p.heading())
			Blank(p.out)
		}
		p.printed = true
		Loading(p.out, p.qualify(fmt.Sprintf(domain.RunStreamStepFmt, event.Step, event.Steps, event.Job), event.Worktree))
	case runlogs.PhaseOutput:
		if len(event.Chunk) == 0 {
			return
		}
		_, _ = p.raw.Write(event.Chunk)
		p.midLine = event.Chunk[len(event.Chunk)-1] != '\n'
	case runlogs.PhaseStarted:
		if event.AlreadyRunning {
			Success(p.out, p.qualify(fmt.Sprintf(domain.RunStreamAlreadyFmt, event.Job), event.Worktree))
			return
		}
		Success(p.out, p.jobLine(jobLineParams{Format: domain.RunStreamStartedFmt, Event: event}))
		p.held(event.Held)
		p.devOrigins(event.DevOrigins)
	case runlogs.PhaseDone:
		Success(p.out, p.jobLine(jobLineParams{Format: domain.RunStreamDoneFmt, Event: event}))
		p.held(event.Held)
	case runlogs.PhaseFailed:
		Error(p.err, p.qualify(event.Reason, event.Worktree))
	case runlogs.PhaseNotice:
		Blank(p.err)
		Callout(p.err, domain.ProxyUnavailableTitle, []string{event.Notice})
	case runlogs.PhaseProbed:
		p.probed(event.Probes)
	case runlogs.PhaseCrashed:
		p.crashed(event)
	case runlogs.PhaseAborted:
		p.aborted(event.Outcome)
	case runlogs.PhaseReady:
		p.ready(event.Outcome)
	}
}

// breakJobLine closes a row a job left open. Its bytes are not barred, so a
// composed line following them on the same row would carry no bar either — and
// would read as part of the job's output rather than as wtm's own.
func (p *RunPrinter) breakJobLine() {
	if !p.midLine {
		return
	}
	_, _ = io.WriteString(p.raw, "\n")
	p.midLine = false
}

// qualify names the worktree a line came from, and leaves it out above a single
// one — where naming it would only repeat what the command was told.
func (p *RunPrinter) qualify(line, worktree string) string {
	if !p.multi || worktree == "" {
		return line
	}
	return fmt.Sprintf(domain.RunStreamWorktreeFmt, line, worktree)
}

// heading names the run: its profile, and how many worktrees it covers when
// that is more than one.
func (p *RunPrinter) heading() string {
	profile := fmt.Sprintf(domain.RunStreamProfileFmt, p.profile)
	if !p.multi {
		return profile
	}
	return fmt.Sprintf(domain.RunStreamWorktreeFmt, profile, fmt.Sprintf(domain.RunStreamWorktreesFmt, p.worktrees))
}

type jobLineParams struct {
	Format string
	Event  runlogs.Event
}

func (p *RunPrinter) jobLine(params jobLineParams) string {
	return JobLine(JobLineParams{
		Label:      p.qualify(fmt.Sprintf(params.Format, params.Event.Job), params.Event.Worktree),
		Ports:      params.Event.Ports,
		URL:        params.Event.URL,
		Hyperlinks: p.hyperlinks,
	})
}

type JobLineParams struct {
	Label string
	Ports map[string]int
	// URL is where the job answers, empty for one that publishes no name.
	URL string
	// Hyperlinks turns the URL into an OSC-8 link.
	Hyperlinks bool
}

// JobLine is how every human surface announces a job: what it is, the ports it
// bound, and where to reach it. Shared so `run up` and `run start` cannot drift
// into saying the same thing two ways.
func JobLine(params JobLineParams) string {
	line := rules.LabelWithPorts(rules.LabelWithPortsParams{
		Label: params.Label,
		Ports: params.Ports,
	})
	if params.URL == "" {
		return line
	}
	return line + domain.RunURLSuffixSep + Hyperlink(HyperlinkParams{
		Text:    params.URL,
		URL:     params.URL,
		Enabled: params.Hyperlinks,
	})
}

// held names the jobs a runner started, under its own line. They have no line
// of their own: they are subprocesses of the one job the daemon holds.
func (p *RunPrinter) held(entries []domain.JobURLEntry) {
	for _, line := range rules.HeldAddressLines(entries) {
		Message(p.out, Indent+line)
	}
}

// devOrigins reports the one line a Next project is missing before its own name
// reaches it. Rendered where the ports report is, for the same reason: it is a
// finding about a job that started fine, not a failure.
func (p *RunPrinter) devOrigins(fixes []domain.DevOriginFix) {
	if len(fixes) == 0 {
		return
	}
	lines := make([]string, 0, len(fixes))
	for _, fix := range fixes {
		lines = append(lines, fix.Line)
	}
	Blank(p.err)
	Callout(p.err, domain.DevOriginsTitle, lines)
}

// crashed corrects a job this run already announced as started. The daemon
// accepts a spawn, not a life: a service binding a busy port is accepted and
// gone a moment later, and a ✓ over a dead process is the one line that must
// never stand.
func (p *RunPrinter) crashed(event runlogs.Event) {
	reason := event.Reason
	if event.ExitCode != nil {
		reason = fmt.Sprintf(domain.RunStreamCrashedCodeFmt, reason, *event.ExitCode)
	}
	Warning(p.err, p.qualify(fmt.Sprintf(domain.RunStreamCrashedFmt, event.Job, reason), event.Worktree))
}

// probed reports only what the check could not confirm: a port that answered
// says nothing the "started" line did not already say.
func (p *RunPrinter) probed(probes []domain.PortProbe) {
	lines := rules.PortProbeLines(probes)
	if len(lines) == 0 {
		return
	}
	Blank(p.err)
	Callout(p.err, domain.PortProbeTitle, lines)
}

// aborted reports the partial state a failed job left behind: where the profile
// stopped, what nothing tore down, and what it never reached. The job's own
// output already streamed past — this says what is left, not why.
func (p *RunPrinter) aborted(outcome runlogs.Outcome) {
	Blank(p.err)
	Warning(p.err, p.qualify(fmt.Sprintf(domain.RunAbortStepFmt, outcome.FailedStep, outcome.Steps, outcome.Failed), outcome.Worktree))

	if len(outcome.Started) > 0 {
		InfoLine(p.err, domain.RunAbortRunningLabel, joinJobNames(outcome.Started))
	}
	if len(outcome.NotStarted) > 0 {
		InfoLine(p.err, domain.RunAbortNotStartedLabel, joinJobNames(outcome.NotStarted))
	}

	Blank(p.err)
	NextStep(p.err, NextStepParams{Command: domain.RunAbortRetryHint, Note: domain.RunAbortRetryNote})
	NextStep(p.err, NextStepParams{Command: domain.RunAbortStopHint, Note: domain.RunAbortStopNote})
}

// ready closes the run with the hint on what to do next. N worktrees each end
// their own sequence, and the hint is about the run rather than about any of
// them, so it is printed once however many reported.
func (p *RunPrinter) ready(outcome runlogs.Outcome) {
	if len(outcome.Started) == 0 || p.readied {
		return
	}
	p.readied = true
	Blank(p.out)
	NextStep(p.out, NextStepParams{Command: domain.RunStreamAttachHint, Note: domain.RunStreamAttachNote})
	NextStep(p.out, NextStepParams{Command: domain.RunStreamStopHint, Note: domain.RunStreamStopNote})
}

// WriteRunOutcomeJSON writes what a run did as the array of job results every
// `run` command emits, with one addition on the job that ended it: the output it
// had written and the code it exited with. A caller reading JSON never saw the
// live stream, and the daemon's message alone ("task migrate failed: exit status
// 1") does not say why.
func WriteRunOutcomeJSON(w io.Writer, outcome runlogs.Outcome) error {
	return WriteJobResultsJSON(w, RunOutcomeResults(outcome))
}

// WriteRunOutcomesJSON writes what a run over one or more worktrees did. The
// shape follows the arity (LUC-198): one worktree answers with the bare array
// of job results, exactly as it always has, and several answer with one
// document each — the only way two jobs called `web` can be told apart.
func WriteRunOutcomesJSON(w io.Writer, outcomes runlogs.Outcomes) error {
	if len(outcomes) <= 1 {
		return WriteRunOutcomeJSON(w, outcomes.One())
	}

	documents := make([]domain.WorktreeRunResult, 0, len(outcomes))
	for _, outcome := range outcomes {
		results := RunOutcomeResults(outcome)
		if results == nil {
			results = []domain.JobActionResult{}
		}
		documents = append(documents, domain.WorktreeRunResult{
			Worktree: outcome.Worktree,
			Path:     outcome.WorkDir,
			Profile:  outcome.Profile,
			Aborted:  outcome.Aborted(),
			Jobs:     results,
		})
	}
	return encodeJSON(w, documents)
}

// RunOutcomeResults is what a run concluded, one entry per job it reached: the
// sequence's own results, with the probes and the failure's detail folded back
// in. `run up` writes the whole slice and `run start` the single entry its one
// job earned, so the two cannot disagree about the same job.
func RunOutcomeResults(outcome runlogs.Outcome) []domain.JobActionResult {
	results := make([]domain.JobActionResult, len(outcome.Results))
	copy(results, outcome.Results)

	probes := map[string][]domain.PortProbe{}
	for _, probe := range outcome.Probes {
		probes[probe.Job] = append(probes[probe.Job], probe)
	}

	for i := range results {
		results[i].Ports = probes[results[i].Name]
		if results[i].Name != outcome.Failed || results[i].Status != domain.JobActionError {
			continue
		}
		results[i].Output = string(outcome.FailedOutput)
		results[i].ExitCode = outcome.FailedExitCode
	}
	return results
}

func joinJobNames(names []string) string {
	return strings.Join(names, domain.RunViewRecapListSep)
}

type RunDownRecapParams struct {
	// Profile names what was stopped, empty for a `run down` that took down
	// everything the worktree had.
	Profile string
	// Results is one entry per worktree, in the order they were emptied.
	Results []domain.WorktreeJobResults
}

// FormatRunDownRecap renders what `run down` took down, in the box `run up`
// leaves behind on its way out. Stopping a profile and starting it are two
// halves of one command; only one of them having a shape made them read as two
// different programs.
func FormatRunDownRecap(params RunDownRecapParams) string {
	var lines []string
	if params.Profile != "" {
		lines = append(lines, fmt.Sprintf(domain.RunViewRecapProfileFmt, params.Profile))
	}
	for _, worktree := range params.Results {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, downRecapBlock(worktree)...)
	}

	hint := NextStepLine(NextStepParams{Command: domain.RunStreamUpHint, Note: domain.RunStreamUpNote})
	body := strings.Join(append(lines, "", hint), "\n")
	// Terminated, like every other Format* body in this package: the frame writes
	// one blank line after what it is given, and a body whose last line has no
	// break of its own swallows it.
	return styles.RenderRecap(styles.IntroParams{
		Width:   domain.RecapWidth,
		Title:   domain.RunViewRecapTitle,
		Body:    body,
		InFrame: true,
	}) + "\n"
}

// downRecapBlock is one worktree's account: what it took down, and what refused
// to go. The worktree is always named — a recap that says which worktree only
// when there are two leaves the reader guessing on the run they do most.
func downRecapBlock(worktree domain.WorktreeJobResults) []string {
	var stopped, failed []string
	for _, result := range worktree.Jobs {
		if result.Status == domain.JobActionError {
			failed = append(failed, result.Name)
			continue
		}
		stopped = append(stopped, result.Name)
	}

	var lines []string
	if worktree.Worktree != "" {
		lines = append(lines, styles.Bold.Render(worktree.Worktree))
	}
	if len(stopped) > 0 {
		lines = append(lines, fmt.Sprintf(domain.RunDownRecapStoppedFmt, joinJobNames(stopped)))
	}
	if len(failed) > 0 {
		lines = append(lines, styles.DangerText.Render(fmt.Sprintf(domain.RunViewRecapFailedFmt, joinJobNames(failed))))
	}
	return lines
}
