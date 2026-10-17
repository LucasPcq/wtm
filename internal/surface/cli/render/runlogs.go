package render

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
	// reach is where each started job is reached, per worktree in the order
	// they reported: the block the run concludes on.
	reach      map[string][]domain.ReachEntry
	reachOrder []string
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
		if p.multi {
			p.heads()
			return
		}
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
			already := domain.RunStreamAlreadyFmt
			if event.Joined {
				already = domain.RunStreamAlreadyJoinedFmt
			}
			p.remember(event)
			Success(p.out, p.jobLine(jobLineParams{Label: fmt.Sprintf(already, event.Job), Event: event}))
			return
		}
		p.remember(event)
		Success(p.out, p.jobLine(jobLineParams{Label: startedLabel(event), Event: event}))
		p.devOrigins(event.DevOrigins)
	case runlogs.PhaseDone:
		Success(p.out, p.jobLine(jobLineParams{Label: fmt.Sprintf(domain.RunStreamDoneFmt, event.Job), Event: event}))
	case runlogs.PhaseFailed:
		Error(p.err, p.qualify(event.Reason, event.Worktree))
	case runlogs.PhaseNotice:
		Blank(p.err)
		Callout(p.err, domain.ProxyUnavailableTitle, []string{event.Notice})
	case runlogs.PhaseWarning:
		Warning(p.err, event.Notice)
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

// heads titles a run over several worktrees once. Its sequences interleave on
// one stream, so it prints no progress line: one would sit above another
// worktree's result.
func (p *RunPrinter) heads() {
	if p.printed || p.profile == "" {
		p.printed = true
		return
	}
	SectionTitle(p.out, p.heading())
	Blank(p.out)
	p.printed = true
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
	Label string
	Event runlogs.Event
}

// startedLabel says where a started job runs when that is not here: a shared
// service this worktree only holds is main's, and stopping this worktree
// leaves it up.
func startedLabel(event runlogs.Event) string {
	switch {
	case event.Joined && event.SharedIn != "" && event.SharedIn != event.Worktree:
		return fmt.Sprintf(domain.RunStreamJoinedInFmt, event.Job, event.SharedIn)
	case event.Joined:
		return fmt.Sprintf(domain.RunStreamJoinedFmt, event.Job)
	}
	return fmt.Sprintf(domain.RunStreamStartedFmt, event.Job)
}

// jobLine carries one address fragment, never the list: the URL, the lone
// port, or how many — the block the run ends on has the rest. A namespace a
// shared start carved rides on the same line.
func (p *RunPrinter) jobLine(params jobLineParams) string {
	event := params.Event
	line := p.qualify(params.Label, event.Worktree)
	entry := reachEntryOf(event)
	if summary := rules.ReachSummary(entry); summary != "" {
		line += domain.ReachDetailSep + p.link(summary)
	}
	if event.Namespace != "" {
		line += domain.ReachDetailSep + fmt.Sprintf(domain.RunStreamNamespaceReadyFmt, event.Namespace)
	}
	return line
}

func reachEntryOf(event runlogs.Event) domain.ReachEntry {
	return rules.ReachEntryFor(rules.ReachEntryParams{
		Job:       event.Job,
		URL:       event.URL,
		Held:      event.Held,
		Ports:     event.Ports,
		Namespace: event.Namespace,
		SharedIn:  event.SharedIn,
	})
}

// remember keeps a started job for the block the run concludes on.
func (p *RunPrinter) remember(event runlogs.Event) {
	if p.reach == nil {
		p.reach = map[string][]domain.ReachEntry{}
	}
	if _, seen := p.reach[event.Worktree]; !seen {
		p.reachOrder = append(p.reachOrder, event.Worktree)
	}
	p.reach[event.Worktree] = append(p.reach[event.Worktree], reachEntryOf(event))
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
	if len(outcome.Started) > 0 {
		p.readied = true
	}
}

// link makes the addresses in text clickable, on a terminal only.
func (p *RunPrinter) link(text string) string {
	if !p.hyperlinks {
		return text
	}
	return rules.LinkURLs(text)
}

func (p *RunPrinter) linkAll(lines []string) []string {
	linked := make([]string, len(lines))
	for i, line := range lines {
		linked[i] = p.link(line)
	}
	return linked
}

// Conclude is the one place the run says where everything is reached: the
// block, then a line for each worktree whose .env is out of step with it, then
// what to do next. It follows every worktree's sequence, so a run over several
// is concluded once.
func (p *RunPrinter) Conclude(warnings []string) {
	if !p.readied {
		return
	}
	worktrees := make([]rules.ReachWorktree, 0, len(p.reachOrder))
	for _, worktree := range p.reachOrder {
		worktrees = append(worktrees, rules.ReachWorktree{Name: worktree, Entries: p.reach[worktree]})
	}
	for _, section := range rules.ReachBlock(rules.ReachBlockParams{Worktrees: worktrees}) {
		Blank(p.out)
		Section(p.out, section.Title, p.linkAll(section.Lines))
	}
	if len(warnings) > 0 {
		Blank(p.out)
		for _, warning := range warnings {
			Warning(p.out, warning)
		}
	}
	Blank(p.out)
	NextStep(p.out, NextStepParams{Command: domain.RunStreamAttachHint, Note: domain.RunStreamAttachNote})
	NextStep(p.out, NextStepParams{Command: domain.RunStreamStopHint, Note: domain.RunStreamStopNote})
}

// RunInterrupted is the account of a start an interrupt cut short: what each
// worktree has left running — the job being started included — and what it
// never reached. Raw body.
func RunInterrupted(w io.Writer, outcomes runlogs.Outcomes) {
	running := false
	for _, outcome := range outcomes {
		Blank(w)
		if outcome.Worktree != "" {
			Message(w, outcome.Worktree)
		}
		if len(outcome.Started) > 0 {
			running = true
			Message(w, fmt.Sprintf(domain.RunViewRecapRunningFmt, strings.Join(outcome.Started, ", ")))
		}
		if len(outcome.NotStarted) > 0 {
			Message(w, fmt.Sprintf(domain.RunViewRecapNotStartedFmt, strings.Join(outcome.NotStarted, ", ")))
		}
		if len(outcome.Started) == 0 {
			Unchanged(w, domain.RunViewRecapNoneRunning)
		}
	}
	if !running {
		return
	}
	Blank(w)
	NextStep(w, NextStepParams{Command: domain.RunStreamStopHint, Note: domain.RunStreamStopNote})
}

// WriteRunOutcomesJSON writes one document per worktree the run reached,
// whatever their number: a caller parses one shape.
func WriteRunOutcomesJSON(w io.Writer, outcomes runlogs.Outcomes) error {
	documents := make([]domain.WorktreeRunResult, 0, len(outcomes))
	for _, outcome := range outcomes {
		results := RunOutcomeResults(outcome)
		if results == nil {
			results = []domain.JobActionResult{}
		}
		documents = append(documents, domain.WorktreeRunResult{
			Branch:      outcome.Worktree,
			Path:        outcome.WorkDir,
			Profile:     outcome.Profile,
			Aborted:     outcome.Aborted(),
			Interrupted: outcome.Interrupted,
			Jobs:        results,
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

	hint := styles.NextStepText(styles.NextStepParams{Command: domain.RunStreamUpHint, Note: domain.RunStreamUpNote})
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
	var stopped, released, failed []string
	for _, result := range worktree.Jobs {
		switch result.Status {
		case domain.JobActionError:
			failed = append(failed, result.Name)
		case domain.JobActionReleased:
			released = append(released, result.Name)
		case domain.JobActionNotRunning:
		default:
			stopped = append(stopped, result.Name)
		}
	}

	var lines []string
	if worktree.Branch != "" {
		lines = append(lines, styles.Bold.Render(worktree.Branch))
	}
	if len(stopped) > 0 {
		lines = append(lines, fmt.Sprintf(domain.RunDownRecapStoppedFmt, joinJobNames(stopped)))
	}
	if len(released) > 0 {
		lines = append(lines, fmt.Sprintf(domain.RunDownRecapReleasedFmt, joinJobNames(released)))
	}
	if len(failed) > 0 {
		lines = append(lines, styles.DangerText.Render(fmt.Sprintf(domain.RunViewRecapFailedFmt, joinJobNames(failed))))
	}
	return lines
}
