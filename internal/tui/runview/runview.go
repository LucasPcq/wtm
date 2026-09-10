package runview

import (
	"context"
	"fmt"
	"io"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/rules"
)

type Params struct {
	Board runlogs.Board
	// Job is selected when the view opens; empty takes the first one.
	Job string
	// Profile names what Start brings up, shown in the header and the recap.
	// Empty for a view that starts nothing, or a run.toml declaring no profile.
	Profile string
	// Worktrees are the worktrees Start covers. The view needs the count before
	// the first event: N sequences end independently, and it is only over when
	// every one of them has reported.
	Worktrees []string
	// Start runs a profile's start sequence while the view is open, reporting to
	// the Sink it is given. Nil for a view that only reads what is already
	// running. Cancelling the context ends the reporting, never the jobs.
	Start runlogs.StartFunc
	// Warnings are what the run has to say about the addresses it publishes,
	// shown in the notice band. They belong to the view rather than to the
	// scrollback under it: a reader who only sees them on the way out has
	// already followed the URL they qualify.
	Warnings []string
	// Detach is what leaving does to a run that is still going; see Detach. The
	// zero value cancels it, which is what a surface with nowhere to report the
	// rest installs.
	Detach Detach
	// Open hands a job's URL to the desktop. Nil leaves the open key without an
	// object, which is what a surface that cannot open a browser installs.
	Open OpenFunc
	// In and Out are the terminal the view takes over. Nil means os.Stdin and
	// os.Stdout, which is every case but one: a dashboard handing the terminal
	// over is given the streams by bubbletea and has to pass them on, or the two
	// programs read the same keyboard at once.
	In  io.Reader
	Out io.Writer
}

// Detach is what happens to a run the reader walks out on. The sequence is the
// client's, not the daemon's — nobody else can finish it — so leaving the view
// hands it to Sink rather than cancelling it, and Notice is called once, first,
// so the surface can say where the rest of the run went before it starts
// arriving. A zero Detach cancels the run instead: a surface with nowhere to
// report the rest cannot honestly claim it continues.
type Detach struct {
	Notice func()
	Sink   runlogs.Sink
	// Await holds Run until the sequence ends. A command has to: its process is
	// the one running the sequence, and returning would exit out from under it.
	// A surface that outlives the view — a dashboard — sets it false and gets
	// its terminal back while the run reports into it.
	Await bool
}

// OpenFunc opens a URL outside the terminal. The view never dials anything
// itself: it names what to open and lets the seam do it.
type OpenFunc func(url string) error

// Model is the run view's root Bubbletea model: a job list, the selected job's
// terminal emulator, and nothing else. What a job is, whether it can be
// attached to and what it has printed are runlogs' answers, never its own.
type Model struct {
	board runlogs.Board
	panes *paneStore
	// msgs carries what the stream readers post; listenCmd is its only reader.
	msgs chan tea.Msg

	width  int
	height int

	jobs     []runlogs.JobView
	selected jobKey
	// wantJob is the job the view was opened on, by name: the flow that opened it
	// named a job, not a worktree's copy of one, so it is resolved to a key on the
	// first listing.
	wantJob string
	offset  int
	err     error

	filtering bool
	filter    string
	// focused reports that the keyboard belongs to the selected job rather than
	// to this view.
	focused bool

	// lastExitKey dates the last exit key the focused job received, which is
	// what turns a second one into a way out.
	lastExitKey time.Time
	// notice is a refusal the view has to answer with rather than act on.
	notice string

	// pending is the job whose pane is being filled — an attach or a history
	// read in flight — so a second one is not started behind it.
	pending jobKey
	// ticking reports whether a redraw clock is already running; see frameCmd
	// for why it outlives the last frame.
	ticking bool

	start runlogs.StartFunc
	// relay is where the run reports: the view while the reader is there, the
	// surface's own reporter once they have left.
	relay *relay
	// finished carries what the run concluded to whoever is left to read it.
	// Buffered, because after the reader leaves nobody may ever be.
	finished chan runFinishedMsg
	onLeave  Detach
	// runDone reports that the sequence is over, which is what makes leaving an
	// ordinary quit rather than a detach.
	runDone bool
	// profile names the run the view is reporting on, for the header and the recap.
	profile string
	open    OpenFunc
	// started reports that a run was asked for, which is what makes a recap
	// worth printing on the way out.
	started  bool
	sequence sequence
	// preview drops everything a hosted view has no room for — the header, the
	// help bar, the job list — and leaves only the pane. It is the same renderer
	// and the same live stream the full view uses, at a panel's size, and it
	// takes no key: acting on a job is what opening the full view is for.
	preview bool
	// following keeps the cursor on the job the sequence is acting on, until the
	// reader takes it themselves.
	following bool
	// dismissed hides the abort report; the outcome it was built from stays.
	dismissed bool
	// warnings are the notice band's last resort — see Params.Warnings.
	warnings []string

	runCtx context.Context
	cancel context.CancelFunc
}

func New(params Params) Model {
	ctx, cancel := context.WithCancel(context.Background())
	panes := newPaneStore(PaneSize{})
	msgs := make(chan tea.Msg, domain.RunViewMsgBuffer)
	return Model{
		board:    params.Board,
		wantJob:  params.Job,
		warnings: params.Warnings,
		profile:  params.Profile,
		panes:    panes,
		msgs:     msgs,
		relay:    newRelay(sink{panes: panes, msgs: msgs, done: ctx.Done()}),
		start:    params.Start,
		onLeave:  params.Detach,
		finished: make(chan runFinishedMsg, 1),
		open:     params.Open,
		started:  params.Start != nil,
		// A run feeds panes from its own goroutine, so the clock has to be
		// running before the first chunk lands.
		ticking:   params.Start != nil,
		following: params.Start != nil,
		sequence: sequence{
			keys:    map[jobKey]bool{},
			states:  map[jobKey]domain.JobStep{},
			reasons: map[string]string{},
			pending: max(len(params.Worktrees), 1),
		},
		runCtx: ctx,
		cancel: cancel,
	}
}

// PreviewParams builds the hosted form of the view.
type PreviewParams struct {
	Board runlogs.Board
	// Job is the job to show. A host changes it with ShowJob as its own cursor
	// moves; the preview never chooses one.
	Job string
}

// NewPreview is the view as a panel holds it: one job's pane, live, with no
// chrome of its own and no keys. The host owns the navigation, and everything
// the reader might act on — focus, filter, scrollback, opening a url — belongs
// to the full view, which is what enter is for.
func NewPreview(params PreviewParams) Model {
	model := New(Params{Board: params.Board, Job: params.Job})
	model.preview = true
	return model
}

// ShowJob points the preview at another job. It is how a host's cursor reaches
// a view that reads no keys of its own.
func (m Model) ShowJob(job string) (Model, tea.Cmd) {
	if job == "" || job == m.selected.job() {
		return m, nil
	}
	m.wantJob = job
	for _, view := range m.jobs {
		if view.Name == job {
			return m.setSelection(viewKey(view))
		}
	}
	return m, nil
}

// SetSize is how a host sizes a preview: the full view learns its size from the
// terminal, a hosted one from whatever panel is holding it.
func (m Model) SetSize(width, height int) (Model, tea.Cmd) {
	if width == m.width && height == m.height {
		return m, nil
	}
	m.width, m.height = width, height
	return m.applySize()
}

// Close releases the panes and the streams behind them. A host that drops a
// preview without calling it leaks a subscription per job it ever showed.
func (m Model) Close() {
	m.cancel()
	m.panes.closeAll()
}

// Run opens the view on the alternate screen and returns what to say once it is
// given back. Leaving it detaches: the jobs it was showing keep running, and a
// start sequence it was reporting on carries on without a reader.
func Run(params Params) (Result, error) {
	model := New(params)
	// Mouse tracking, or the wheel falls through to the host terminal and writes
	// escape sequences over the view instead of scrolling the pane.
	options := []tea.ProgramOption{tea.WithAltScreen(), tea.WithMouseCellMotion()}
	if params.In != nil {
		options = append(options, tea.WithInput(params.In))
	}
	if params.Out != nil {
		options = append(options, tea.WithOutput(params.Out))
	}
	final, err := tea.NewProgram(model, options...).Run()
	if err != nil {
		model.cancel()
		model.panes.closeAll()
		return Result{}, fmt.Errorf("run view: %w", err)
	}
	last, ok := final.(Model)
	if !ok {
		return Result{}, nil
	}
	return last.awaitDetached(), nil
}

// awaitDetached finishes a run the reader walked out on. It runs with the
// terminal already given back, which is the whole reason it is here rather than
// inside the program: the surface's reporter writes to the scrollback, and the
// alternate screen has to be gone first.
func (m Model) awaitDetached() Result {
	if m.runDone || m.start == nil || m.onLeave.Sink == nil {
		m.cancel()
		return m.result()
	}
	// The context stays live: cancelling it is what would abort the very
	// sequence this is handing over. What it still holds — a parked listener and
	// the redraw clock — ends with the run, which releases it below or, for a
	// surface that does not wait, when the sequence posts its result.
	if m.onLeave.Notice != nil {
		m.onLeave.Notice()
	}
	m.relay.redirect(m.onLeave.Sink)

	if !m.onLeave.Await {
		go func() { <-m.finished; m.cancel() }()
		return Result{Detached: true}
	}
	defer m.cancel()
	done := <-m.finished
	// The recap belongs to the view, and the view is gone: what the run
	// concluded was reported line by line as it happened.
	return Result{Outcomes: done.outcomes, Detached: true}
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.refreshCmd(), pollCmd(), m.listenCmd()}
	if m.start != nil {
		cmds = append(cmds, m.startCmd(), m.frameCmd())
	}
	return tea.Batch(cmds...)
}

// startCmd runs the start sequence off the render loop. Its events come back
// through the sink, which writes a job's output straight into that job's pane
// and posts only the phases the view has to draw.
func (m Model) startCmd() tea.Cmd {
	start, ctx, relay, finished := m.start, m.runCtx, m.relay, m.finished
	return func() tea.Msg {
		outcomes, err := start(ctx, relay)
		done := runFinishedMsg{outcomes: outcomes, err: err}
		// The view may be gone; Run is then the only reader left, and it is not
		// listening yet. One slot, never blocking either of them.
		finished <- done
		return done
	}
}

type jobsMsg struct {
	jobs []runlogs.JobView
	err  error
}

type attachedMsg struct {
	key    jobKey
	stream runlogs.Stream
	// size is what the job's PTY was sized to as the subscription opened, which
	// the view may have outgrown while the daemon was answering.
	size PaneSize
	err  error
}

type historyMsg struct {
	key   jobKey
	lines []string
	err   error
}

type streamEndedMsg struct{ key jobKey }

type streamErrMsg struct {
	key jobKey
	err error
}

type frameMsg struct{}

type pollMsg struct{}

func (m Model) refreshCmd() tea.Cmd {
	board := m.board
	return func() tea.Msg {
		if err := board.Refresh(); err != nil {
			return jobsMsg{jobs: board.Jobs(), err: err}
		}
		return jobsMsg{jobs: board.Jobs()}
	}
}

type attachParams struct {
	Key  jobKey
	Size PaneSize
}

func (m Model) attachCmd(params attachParams) tea.Cmd {
	board := m.board
	return func() tea.Msg {
		stream, err := board.Attach(runlogs.AttachParams{
			Job:     params.Key.job(),
			WorkDir: params.Key.workDir(),
			Size:    runlogs.Size{Cols: params.Size.Cols, Rows: params.Size.Rows},
		})
		return attachedMsg{key: params.Key, stream: stream, size: params.Size, err: err}
	}
}

func (m Model) historyCmd(key jobKey) tea.Cmd {
	board := m.board
	return func() tea.Msg {
		lines, err := board.History(runlogs.HistoryParams{Job: key.job(), WorkDir: key.workDir()})
		return historyMsg{key: key, lines: lines, err: err}
	}
}

func pollCmd() tea.Cmd {
	return tea.Tick(domain.RunViewPollSeconds*time.Second, func(time.Time) tea.Msg { return pollMsg{} })
}

// frameCmd answers only with a frame the screen does not already hold: a tick
// with nothing written since the last one produces no message at all, and
// Bubbletea skips View() entirely for a command that returns nil. The view has
// no animation of its own, so a frame no byte asked for redraws the same pixels.
//
// It runs until the view does. Once the last stream has ended nothing raises
// the flag again, so the loop parks on the ticker rather than reporting itself
// out — which is also why m.ticking may stay true with no frame in sight.
func (m Model) frameCmd() tea.Cmd {
	panes, done, interval := m.panes, m.runCtx.Done(), m.frameInterval()
	return func() tea.Msg {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return nil
			case <-ticker.C:
				if panes.takeDirty() {
					return frameMsg{}
				}
			}
		}
	}
}

func (m Model) frameInterval() time.Duration {
	if m.preview {
		return time.Second / domain.RunViewPreviewRenderFPS
	}
	return time.Second / domain.RunViewRenderFPS
}

// listenCmd takes the next message the stream readers and the run posted, and
// gives up when the view does: the channel is never closed, so a reader parked
// on it would outlive the program.
func (m Model) listenCmd() tea.Cmd {
	msgs, done := m.msgs, m.runCtx.Done()
	return func() tea.Msg {
		select {
		case msg := <-msgs:
			return msg
		case <-done:
			return nil
		}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m.applySize()

	case tea.KeyMsg:
		// A hosted preview reads no key: the panel holding it owns the same
		// arrows, and acting on a job is what opening the full view is for. The
		// refusal lives here rather than in the host, so a second host cannot
		// forget it.
		if m.preview {
			return m, nil
		}
		return m.handleKey(msg)

	case tea.MouseMsg:
		if m.preview {
			return m, nil
		}
		return m.handleMouse(msg)

	case jobsMsg:
		return m.applyJobs(msg)

	case attachedMsg:
		return m.applyAttached(msg)

	case historyMsg:
		return m.applyHistory(msg)

	case eventMsg:
		return m.applyEvent(msg)

	case runFinishedMsg:
		return m.applyRunFinished(msg)

	case openFailedMsg:
		m.notice = fmt.Sprintf(domain.RunViewOpenFailedFmt, msg.err)
		return m, nil

	case streamErrMsg:
		m.err = msg.err
		return m, nil

	case streamEndedMsg:
		return m.applyStreamEnded(msg)

	case pollMsg:
		return m, tea.Batch(m.refreshCmd(), pollCmd())

	case frameMsg:
		if !m.panes.hasStream() && !m.sequence.active {
			m.ticking = false
			return m, nil
		}
		return m, m.frameCmd()
	}

	return m, nil
}

// applySize gives the emulators the room the layout just measured, and the
// jobs behind them the same size. x/vt never reflows: only the job's own
// process can redraw at the new width, and it does not know about it until its
// PTY does.
func (m Model) applySize() (Model, tea.Cmd) {
	size := m.paneSize()
	return m, resizeCmd(resizeParams{Streams: m.panes.resize(size), Size: size})
}

// resyncSize re-sizes the emulators when the abort report's occupation of the
// frame changed. The band takes its rows from the body, and a pane fed at one
// size while drawn shorter shows its oldest rows: the output of the job that
// just failed is exactly what would fall off the bottom.
func (m Model) resyncSize(noticeLines int) (Model, tea.Cmd) {
	if len(m.report()) == noticeLines {
		return m, nil
	}
	return m.applySize()
}

func (m Model) paneSize() PaneSize {
	layout := m.layout()
	return PaneSize{Cols: layout.PaneCols, Rows: layout.PaneRows}
}

func (m Model) layout() domain.RunViewLayout {
	if m.preview {
		return rules.PreviewLayout(rules.PreviewLayoutParams{Width: m.width, Height: m.height})
	}
	return rules.ComputeRunViewLayout(rules.RunViewLayoutParams{
		Width:       m.width,
		Height:      m.height,
		NoticeLines: len(m.report()),
	})
}

func (m Model) applyJobs(msg jobsMsg) (Model, tea.Cmd) {
	m.jobs, m.err = msg.jobs, msg.err
	return m.setSelection(m.resolveSelection())
}

// resolveSelection keeps the cursor on the job it was on for as long as that
// job is both declared and shown, and falls back to the first one visible. The
// name the view was opened on only ever answers once: from then on the cursor
// belongs to the reader.
func (m Model) resolveSelection() jobKey {
	visible := m.visible()
	for _, view := range visible {
		if viewKey(view) == m.selected {
			return m.selected
		}
	}
	if len(visible) == 0 {
		return ""
	}
	for _, view := range visible {
		if view.Name == m.wantJob {
			return viewKey(view)
		}
	}
	return viewKey(visible[0])
}

// fillSelectedPane binds the selected job to the one thing that can fill its
// pane: its live stream while it has one, the log file otherwise. Only the
// selected job is ever attached — a second subscription would double a job's
// output on screen and size its PTY behind the pane that is being read.
func (m Model) fillSelectedPane() (Model, tea.Cmd) {
	view, found := m.selectedView()
	key := viewKey(view)
	if !found || m.pending == key {
		return m, nil
	}

	// A job the run is starting is already writing into its pane through the
	// sequence; attaching would replay what it wrote and show it twice.
	if m.sequenceHolds(key) {
		return m, nil
	}

	entry, held := m.panes.entry(key)
	if view.Attachable {
		if held && entry.source == sourceLive && entry.stream != nil {
			return m, nil
		}
		m.pending = key
		return m, m.attachCmd(attachParams{Key: key, Size: m.paneSize()})
	}

	if held {
		return m, nil
	}
	m.pending = key
	return m, m.historyCmd(key)
}

func (m Model) kindOf(key jobKey) domain.JobKind {
	for _, view := range m.jobs {
		if viewKey(view) == key {
			return view.Kind
		}
	}
	return ""
}

// sequenceHolds reports that the run is writing into the job's pane itself.
// Those bytes are nowhere else yet: no subscription carries them and the log
// file is still being written, so the pane is the only copy.
func (m Model) sequenceHolds(key jobKey) bool {
	return m.sequence.active && m.sequence.keys[key]
}

func (m Model) applyAttached(msg attachedMsg) (Model, tea.Cmd) {
	if m.pending == msg.key {
		m.pending = ""
	}
	// A stream that arrives for a job the cursor has left is closed on the spot:
	// nothing will read it, and an unread subscription holds the job's output
	// queue and its PTY size.
	if msg.key != m.selected {
		if msg.stream != nil {
			msg.stream.Close()
		}
		return m, nil
	}
	if msg.err != nil {
		m.err = msg.err
		return m, nil
	}

	m.err = nil
	pane := m.panes.attach(attachPaneParams{Key: msg.key, Stream: msg.stream})
	go readStream(readParams{
		Key:          msg.key,
		Stream:       msg.stream,
		Pane:         pane,
		Msgs:         m.msgs,
		Done:         m.runCtx.Done(),
		NormalizeEOL: rules.RunsOnPipe(m.kindOf(msg.key)),
	})

	model, tick := m.startTicking()
	// The window may have moved while the daemon was answering, and the size the
	// subscription carried is the one it opened with.
	if size := pane.Size(); size != msg.size {
		return model, tea.Batch(tick, resizeCmd(resizeParams{Streams: []runlogs.Stream{msg.stream}, Size: size}))
	}
	return model, tick
}

// applyStreamEnded is the job's output running out. The pane keeps what it
// printed, the subscription goes, and so does the keyboard: there is nothing
// left to type into.
func (m Model) applyStreamEnded(msg streamEndedMsg) (Model, tea.Cmd) {
	m.panes.endStream(msg.key)
	if m.pending == msg.key {
		m.pending = ""
	}
	if m.selected == msg.key {
		m.focused = false
	}
	return m, tea.Batch(m.refreshCmd(), m.listenCmd())
}

func (m Model) applyHistory(msg historyMsg) (Model, tea.Cmd) {
	if m.pending == msg.key {
		m.pending = ""
	}
	if msg.err != nil {
		m.err = msg.err
		return m, nil
	}
	m.err = nil
	m.panes.writeLines(writeLinesParams{Key: msg.key, Lines: msg.lines})
	return m, nil
}

// startTicking schedules the redraw clock unless one is already running: bytes
// are taken as they arrive and drawn on the clock, never once per chunk.
func (m Model) startTicking() (Model, tea.Cmd) {
	if m.ticking {
		return m, nil
	}
	m.ticking = true
	return m, m.frameCmd()
}

type writeParams struct {
	Key    jobKey
	Stream runlogs.Stream
	Bytes  []byte
}

// writeCmd feeds a job's stdin off the render loop: the bytes travel to the
// daemon, and a keystroke is not worth blocking a frame on.
func writeCmd(params writeParams) tea.Cmd {
	return func() tea.Msg {
		if err := params.Stream.Write(params.Bytes); err != nil {
			return streamErrMsg{key: params.Key, err: err}
		}
		return nil
	}
}

type resizeParams struct {
	Streams []runlogs.Stream
	Size    PaneSize
}

// resizeCmd sizes the PTYs behind the panes that moved. A refusal is not worth
// reporting: a stream closed between the measurement and the round-trip is a
// pane nobody is looking at any more.
func resizeCmd(params resizeParams) tea.Cmd {
	if len(params.Streams) == 0 {
		return nil
	}
	return func() tea.Msg {
		for _, stream := range params.Streams {
			stream.Resize(runlogs.Size{Cols: params.Size.Cols, Rows: params.Size.Rows})
		}
		return nil
	}
}

type readParams struct {
	Key    jobKey
	Stream runlogs.Stream
	Pane   *Pane
	Msgs   chan<- tea.Msg
	// NormalizeEOL terminates the job's bare LFs the way a PTY would, for a job
	// whose output reaches here off a pipe instead.
	NormalizeEOL bool
	// Done is the view being gone, which is the only thing that lets a reader
	// stop waiting for its last message to be taken.
	Done <-chan struct{}
}

// readStream drains a job's output into its pane as it arrives. It draws
// nothing: the model's tick is what paces the screen, so a job printing
// megabytes costs writes into the emulator rather than renders of it.
func readStream(params readParams) {
	pendingCR := false
	for chunk := range params.Stream.Chunks() {
		if params.NormalizeEOL {
			normalized := rules.NormalizeEOL(rules.NormalizeEOLParams{Chunk: chunk, PendingCR: pendingCR})
			chunk, pendingCR = normalized.Chunk, normalized.PendingCR
		}
		params.Pane.Write(chunk)
	}
	// Nothing else reports the end of a stream — the poll only re-reads the job
	// list — so dropping it on a busy model would leave the subscription
	// registered for the life of the view: a dead pane redrawn at 30 fps, and a
	// focus that writes keystrokes into a closed stream.
	select {
	case params.Msgs <- streamEndedMsg{key: params.Key}:
	case <-params.Done:
	}
}

// worktreeCount is how many worktrees the view covers, which only the header
// reads: naming a worktree is not gated on there being two. A view opened from a
// picker shows one the reader chose rather than typed, and leaving it unnamed on
// the run they do most is where it is missed hardest.
func (m Model) worktreeCount() int {
	seen := map[string]bool{}
	for _, view := range m.jobs {
		seen[view.WorkDir] = true
	}
	return len(seen)
}

// qualify names the worktree a line belongs to, whenever it has one to name.
func (m Model) qualify(line, worktree string) string {
	if worktree == "" {
		return line
	}
	return fmt.Sprintf(domain.RunStreamWorktreeFmt, line, worktree)
}

func (m Model) visible() []runlogs.JobView {
	views := make([]runlogs.JobView, 0, len(m.jobs))
	for _, view := range m.jobs {
		if rules.MatchesJobFilter(view.Name, m.filter) {
			views = append(views, view)
		}
	}
	return views
}

func (m Model) selectedView() (runlogs.JobView, bool) {
	for _, view := range m.jobs {
		if viewKey(view) == m.selected {
			return view, true
		}
	}
	return runlogs.JobView{}, false
}

func (m Model) selectedIndex() int {
	for index, view := range m.visible() {
		if viewKey(view) == m.selected {
			return index
		}
	}
	return 0
}
