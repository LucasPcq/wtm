package runview

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	pollEvery = 2 * time.Second
	frameRate = 30
	// focusExitWindow is how close behind the first esc the second must come to
	// give the keyboard back: a lone esc is the job's.
	focusExitWindow = 600 * time.Millisecond
	scrollLines     = 3
)

// A binding is what fills the pane: the job, the start it is of, and whether
// it is followed live or read from its log. The pane is filled once per
// binding: a failure is not tried again until the binding changes, the job
// started again, ended, or selected anew.
type binding struct {
	key     key
	started time.Time
	live    bool
}

// A model is the view's Bubble Tea model: the jobs as the daemon last listed
// them, the list's cursor and filter, the band of notices, and the pane of the
// job selected.
type model struct {
	ctx    context.Context
	params Params
	st     *output.Styles
	now    func() time.Time
	msgs   chan tea.Msg
	wake   chan struct{} // a pane was written: a frame is due

	width, height int
	jobs          []run.Job
	listedOnce    bool
	up            map[key]bool // the jobs up at the last listing, whose crash is a notice
	err           error        // the listing's

	selected  key
	offset    int
	filtering bool
	filter    string
	reaching  bool
	focused   bool
	lastEsc   time.Time
	said      string // what the view answers a key with, until the next key

	notices   []string
	dismissed bool

	pane    *pane
	conn    net.Conn
	bound   binding
	pending bool
	failed  error // why the binding could not fill the pane: said until it changes
	ended   bool  // the live stream of the binding ended: its pane is kept
	// again is the binding attached again once its stream ended while its job
	// ran on: a second end keeps the pane, so a stream that ends at once is not
	// attached in a loop.
	again binding
}

// newModel returns the view of params, its first listing not yet asked.
func newModel(ctx context.Context, params Params) *model {
	return &model{
		ctx:     ctx,
		params:  params,
		st:      output.StylesOf(params.Out),
		now:     time.Now,
		msgs:    make(chan tea.Msg, 1),
		wake:    make(chan struct{}, 1),
		up:      map[key]bool{},
		notices: params.Notices,
	}
}

// ----------------------------------------------------------------------------
// The messages

type (
	jobsMsg struct {
		jobs []run.Job
		err  error
	}
	attachedMsg struct {
		bound binding
		at    size // the size the job's terminal was given, which the frame may have left since
		conn  net.Conn
		err   error
	}
	historyMsg struct {
		bound binding
		lines []string
		err   error
	}
	endedMsg struct {
		conn net.Conn
		err  error // why the stream broke; nil when it ended
	}
	saidMsg  struct{ text string }
	frameMsg struct{}
	pollMsg  struct{}
)

// Init asks the first listing, starts the poll and listens for the streams' ends.
func (m *model) Init() tea.Cmd {
	return tea.Batch(m.list(), poll(), m.listen(), m.frame())
}

// Update answers one message.
func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, tea.Batch(m.resize(), m.fill())
	case tea.KeyMsg:
		return m, m.key(msg)
	case tea.MouseMsg:
		return m, m.mouse(msg)
	case jobsMsg:
		return m, m.listed(msg)
	case attachedMsg:
		return m, m.attached(msg)
	case historyMsg:
		m.historyRead(msg)
		return m, nil
	case endedMsg:
		return m, tea.Batch(m.streamEnded(msg), m.listen(), m.list())
	case saidMsg:
		m.said = msg.text
		return m, nil
	case pollMsg:
		return m, tea.Batch(m.list(), poll())
	case frameMsg:
		return m, m.frame()
	}
	return m, nil
}

// close lets go of the job attached and of its pane.
func (m *model) close() {
	if m.conn != nil {
		_ = m.conn.Close()
		m.conn = nil
	}
	if m.pane != nil {
		m.pane.close()
	}
}

// ----------------------------------------------------------------------------
// The listing

func (m *model) list() tea.Cmd {
	ctx, daemon := m.ctx, m.params.Daemon
	return func() tea.Msg {
		jobs, err := daemon.Jobs(ctx)
		if skewed, ok := errors.AsType[*run.Skewed](err); ok && skewed != nil {
			err = nil
		}
		return jobsMsg{jobs: jobs, err: err}
	}
}

func poll() tea.Cmd {
	return tea.Tick(pollEvery, func(time.Time) tea.Msg { return pollMsg{} })
}

// listed takes a listing in: the jobs, the crashes since the last one as
// notices, the cursor kept on its job while it is listed.
func (m *model) listed(msg jobsMsg) tea.Cmd {
	if msg.err != nil && msg.jobs == nil {
		m.err = msg.err
		return nil
	}
	m.err = msg.err
	m.jobs = msg.jobs
	m.notice(entries(m.params, m.jobs, ""))
	m.listedOnce = true

	// A notice takes its rows from the pane: the pane and the job's terminal follow.
	return tea.Batch(m.resize(), m.selectKey(m.resolve()))
}

// notice adds to the band each job that was up at the last listing and has
// crashed or failed since; a notice brings the band back once dismissed.
func (m *model) notice(list []entry) {
	up := map[key]bool{}
	for _, e := range list {
		if e.job.Up {
			up[e.key] = true
		}
		if m.listedOnce && m.up[e.key] && markOf(e) == markFailed {
			m.say(fmt.Sprintf(crashedFmt, e.name, e.branch, m.state(e)))
		}
	}
	m.up = up
}

// say adds text to the band's notices, the band shown again if it was dismissed.
func (m *model) say(text string) {
	m.notices = append(m.notices, text)
	m.dismissed = false
}

// label names the job of k by its worktree's branch.
func (m *model) label(k key) string {
	for _, wt := range m.params.Worktrees {
		if wt.Path == k.dir {
			return k.name + separator + branchOf(wt)
		}
	}
	return k.name
}

// visible is the list as filtered.
func (m *model) visible() []entry {
	return entries(m.params, m.jobs, m.filter)
}

// resolve is the job the cursor stays on: the one it was on while listed,
// else the job asked first, else the first one.
func (m *model) resolve() key {
	list := m.visible()
	for _, e := range list {
		if e.key == m.selected {
			return e.key
		}
	}
	if len(list) == 0 {
		return key{}
	}
	if m.selected == (key{}) {
		for _, e := range list {
			if e.name == m.params.Job {
				return e.key
			}
		}
	}
	return list[0].key
}

func (m *model) selectedEntry() (entry, bool) {
	for _, e := range m.visible() {
		if e.key == m.selected {
			return e, true
		}
	}
	return entry{}, false
}

// selectKey moves the cursor to k, scrolls the list to keep it in sight and
// fills the pane for it.
func (m *model) selectKey(k key) tea.Cmd {
	if k != m.selected {
		m.selected, m.focused = k, false
	}
	all := rows(m.visible())
	m.offset = scrollOffset(rowIndex(all, k), len(all), m.listViewport(), m.offset)
	return m.fill()
}

func rowIndex(all []row, k key) int {
	for i, r := range all {
		if r.entry != nil && r.entry.key == k {
			return i
		}
	}
	return 0
}

// listViewport is how many rows the list scrolls in: one is kept for the
// heading it pins once its own scrolled away.
func (m *model) listViewport() int {
	return max(m.layout().listRows-1, 1)
}

// ----------------------------------------------------------------------------
// The pane

// fill binds the pane to the job selected: its live stream while it runs, its
// log once it ended, nothing for a job not started. Only the job selected is
// attached: a second client would size its terminal behind a pane nobody reads.
func (m *model) fill() tea.Cmd {
	e, found := m.selectedEntry()
	if !found || !e.listed || m.width == 0 {
		m.release()
		return nil
	}
	want := binding{key: e.key, started: e.job.StartedAt, live: attachable(e)}
	// A stream that ended keeps its pane until the job starts again; while the
	// job runs on (the daemon let a client that fell behind go), it is attached
	// again, once per start.
	ended := m.ended && m.bound.key == e.key
	if (want == m.bound && !m.ended) || (ended && (!want.live || want == m.again)) {
		return nil
	}
	if ended && want == m.bound {
		m.again = want
	}

	m.release()
	m.bound, m.pending = want, true
	m.pane = newPane(paneParams{at: m.paneSize(), crlf: onPipe(e), wake: m.wake})
	if want.live {
		return m.attach(want)
	}
	return m.history(want)
}

// release drops the pane and the stream that fed it.
func (m *model) release() {
	m.close()
	m.pane, m.bound, m.pending, m.failed, m.ended, m.focused = nil, binding{}, false, nil, false, false
}

func (m *model) attach(b binding) tea.Cmd {
	ctx, daemon, at := m.ctx, m.params.Daemon, m.paneSize()
	return func() tea.Msg {
		conn, err := daemon.Attach(ctx, run.Window{Name: b.key.name, WorkDir: b.key.dir, Cols: at.cols, Rows: at.rows})
		// The view left while the daemon answered: nobody would close it.
		if err == nil && ctx.Err() != nil {
			_ = conn.Close()
			return nil
		}
		return attachedMsg{bound: b, at: at, conn: conn, err: err}
	}
}

func (m *model) attached(msg attachedMsg) tea.Cmd {
	if msg.bound != m.bound || m.pane == nil {
		if msg.conn != nil {
			_ = msg.conn.Close()
		}
		return nil
	}
	m.pending = false
	if msg.err != nil {
		m.failed = msg.err
		return nil
	}

	m.conn = msg.conn
	go drain(drainParams{conn: msg.conn, pane: m.pane, msgs: m.msgs, done: m.ctx.Done()})

	if at := m.pane.size; at != sized(msg.at) {
		return m.resizeJob(at)
	}
	return nil
}

type drainParams struct {
	conn net.Conn
	pane *pane
	msgs chan<- tea.Msg
	done <-chan struct{}
}

// drain writes what the job prints into its pane until the stream ends, then
// says so, and why when it broke: nothing else would tell the view a job
// stopped printing. A panic while it reads (the emulator is pre-release) ends
// the stream rather than the process, which would leave the terminal raw and
// on the alternate screen.
func drain(p drainParams) {
	ended := endedMsg{conn: p.conn}
	defer func() {
		if r := recover(); r != nil {
			ended.err = fmt.Errorf("reading the stream panicked: %v", r)
		}
		select {
		case p.msgs <- ended:
		case <-p.done:
		}
	}()
	_, ended.err = io.Copy(p.pane, p.conn)
}

func (m *model) listen() tea.Cmd {
	msgs, done := m.msgs, m.ctx.Done()
	return func() tea.Msg {
		select {
		case msg := <-msgs:
			return msg
		case <-done:
			return nil
		}
	}
}

// streamEnded keeps the pane of a job whose stream ended, what it printed last
// being what is worth reading, and takes the keyboard back.
func (m *model) streamEnded(msg endedMsg) tea.Cmd {
	_ = msg.conn.Close()
	if msg.conn != m.conn {
		return nil
	}
	m.conn, m.ended, m.focused = nil, true, false
	if msg.err == nil {
		return nil
	}
	m.say(fmt.Sprintf(streamBrokeFmt, m.label(m.bound.key), msg.err))
	return m.resize()
}

func (m *model) history(b binding) tea.Cmd {
	ctx, daemon := m.ctx, m.params.Daemon
	return func() tea.Msg {
		var lines []string
		_, err := daemon.Lines(ctx, run.Tail{Name: b.key.name, WorkDir: b.key.dir, Lines: run.MaxTail}, func(line run.Line) {
			lines = append(lines, line.Text)
		})
		return historyMsg{bound: b, lines: lines, err: err}
	}
}

func (m *model) historyRead(msg historyMsg) {
	if msg.bound != m.bound || m.pane == nil {
		return
	}
	m.pending = false
	if msg.err != nil && len(msg.lines) == 0 {
		var refused *run.Error
		if !errors.As(msg.err, &refused) || refused.Code != run.CodeJobNotFound {
			m.failed = msg.err
		}
		return
	}
	_, _ = m.pane.Write([]byte(strings.Join(msg.lines, "\r\n")))
}

// frame waits for a pane to be written, then gives a frame a moment later, so
// that a job printing fast is drawn at the frame rate, never once per write.
func (m *model) frame() tea.Cmd {
	wake, done := m.wake, m.ctx.Done()
	return func() tea.Msg {
		select {
		case <-wake:
		case <-done:
			return nil
		}
		time.Sleep(time.Second / frameRate)
		return frameMsg{}
	}
}

func (m *model) paneSize() size {
	l := m.layout()
	return size{cols: l.paneCols, rows: l.paneRows}
}

// resize sizes the pane to the frame, and the job's terminal behind it.
func (m *model) resize() tea.Cmd {
	if m.pane == nil || !m.pane.resize(m.paneSize()) || m.conn == nil {
		return nil
	}
	return m.resizeJob(m.pane.size)
}

// resizeJob sizes the terminal of the job attached; a job under none refuses,
// which changes nothing worth saying.
func (m *model) resizeJob(at size) tea.Cmd {
	ctx, daemon, b := m.ctx, m.params.Daemon, m.bound
	return func() tea.Msg {
		_ = daemon.Resize(ctx, run.Window{Name: b.key.name, WorkDir: b.key.dir, Cols: at.cols, Rows: at.rows})
		return nil
	}
}

// band is the notices on screen: none once dismissed.
func (m *model) band() []string {
	if m.dismissed || len(m.notices) == 0 {
		return nil
	}
	return append(append([]string{}, m.notices...), bandDismiss)
}

func (m *model) layout() layout {
	return layoutAt(m.width, m.height, len(m.band()))
}
