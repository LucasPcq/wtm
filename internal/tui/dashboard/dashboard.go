// Package dashboard renders `wtm ui`: a full-screen view of the repository's
// worktrees, and the surface the create and clean flows are run from. It draws
// and it asks; what a run does is internal/flow's business, never its own.
package dashboard

import (
	"fmt"
	"maps"
	"path/filepath"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/runconfig"
	"github.com/LucasPcq/wtm/internal/service/runjobs"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/tui/components"
	"github.com/LucasPcq/wtm/internal/tui/runview"
	"github.com/LucasPcq/wtm/internal/tui/worktreepicker"
)

// RunParams holds what the dashboard needs to keep itself fed. The PR loader is
// injected so the slow `gh` call stays a surface concern, as in the pickers.
type RunParams struct {
	ProjectDir string
	StateDir   string
	// Cwd is the directory the shell was in when it launched `wtm ui` — not
	// necessarily ProjectDir, which LoadConfig may have resolved upward. It is
	// what the active-worktree match is run against.
	Cwd      string
	Config   domain.Config
	PRLoader worktreepicker.PRLoaderFunc
	// PROpener launches the given PR number in the browser (ghservice.OpenPR,
	// wired with ProjectDir). Injected the same way PRLoader is, so a test can
	// exercise the REVIEW section's click without shelling out to a real gh.
	PROpener func(number int) error
	// JobsLoader reads the run daemon's index. wake says whether waking a
	// sleeping daemon is worth it: every explicit path reads with, the poll
	// reads without, and known reports whether the index could be read at all.
	// Injected like PRLoader, so a test never dials a real socket.
	JobsLoader func(wake bool) (jobs []domain.JobInfo, known bool)
	// URLOpener hands a job's address to the desktop's own opener. Injected like
	// PROpener so a click on a RUN row is asserted without launching a browser.
	URLOpener func(url string) error
	// AddressLoader is where the named worktrees' jobs answer. It is only ever
	// given worktrees that already have a job up: BranchEnv allocates an ordinal
	// the first time it is asked for one. It takes the run.toml the poll already
	// read, so the file is not read twice a poll.
	AddressLoader func(request AddressRequest) domain.RunAddresses
	// LogsLoader reads back a job's persisted output for the detail panel's
	// logs view. Injected like JobsLoader, so a test never opens a real board.
	LogsLoader func(logsRequest) ([]string, error)
	// BoardLoader opens the board a live preview attaches through. Nil leaves the
	// panel on LogsLoader's persisted tail, which is what a test installs.
	BoardLoader func(logsRequest) runlogs.Board
	// TraceLoader names, per branch, the jobs that left output in that worktree.
	// It is asked about every worktree rather than only the ones with a job up:
	// a job that left a trace is precisely one the daemon no longer holds, and
	// asking only about live worktrees would hide every finished task and every
	// crash — the two things one opens this panel for.
	TraceLoader func(branches []string) map[string]map[string]bool
	// Version is the running wtm version and UpgradeLatest the newer release the
	// last passive check found, or "" when there is none. Both are resolved by
	// the command layer: the dashboard renders them, it decides nothing.
	Version       string
	UpgradeLatest string
}

// OutputLineMsg appends one line to the bottom output panel. Every phase of a
// running flow — hook output included — reaches the panel through it.
type OutputLineMsg struct{ Text string }

// openPRMsg carries the outcome of launching a PR in the browser. err is nil
// on success — opening the tab is its own feedback, so nothing is posted to
// the output panel unless the launch itself failed.
type openPRMsg struct{ err error }

// openURLMsg is openPRMsg for a job's address: the opened tab is its own
// feedback, so only a failed launch reaches the output panel.
type openURLMsg struct{ err error }

type worktreesMsg struct {
	statuses  []domain.WorktreeStatus
	parents   map[string]string
	fetchedAt time.Time
	err       error
}

// jobsMsg carries the run daemon's index and the run.toml it is read against:
// what each worktree has up, and what the project declares it could run. It
// never fails the dashboard — a daemon that is not listening simply means
// nothing is running, which is the answer.
type AddressRequest struct {
	Branches []string
	Config   domain.RunConfig
}

// addressesMsg lands the addresses the poll asked for. It is its own message
// rather than a field of jobsMsg because the two reads cannot be ordered: Init
// loads the worktrees and the jobs in parallel, and whichever answers last is
// the one that knows enough to ask.
// tracesMsg lands what each worktree has left on disk. Its own message, like
// addressesMsg and for the same reason: it is read off the worktrees, which may
// land after the jobs.
type tracesMsg struct {
	logged map[string]map[string]bool
}

type addressesMsg struct {
	addresses map[string]map[string]domain.JobAddress
	// notes is what has to be said about an address, keyed by branch: a
	// worktree served its ports because its .env was never settled on the names
	// it publishes says so, or the reader wonders why it alone has no name.
	notes map[string]string
	// portAddressed keys the worktrees served their ports rather than their
	// names. The preview's own board needs the verdict, not the line: it builds
	// its addresses itself and has to reach the same one.
	portAddressed map[string]bool
}

type jobsMsg struct {
	jobs    []domain.JobInfo
	running map[string]int
	config  domain.RunConfig
	// known is false when the daemon could not be asked while its index still
	// holds jobs: what runs is then unknown, which is not the same answer as
	// nothing running, and the counts already on screen are kept.
	known bool
}

type prsMsg struct {
	prs  []domain.PRInfo
	conn domain.GHConnection
}

type pollMsg struct{}

type gitPollMsg struct{}

// tabSlideTickMsg redraws while the tab rule is sliding. The handler is where
// the sequence ends: it re-arms only while the slide is still short of its
// duration, so an idle dashboard schedules no further ticks once it settles.
type tabSlideTickMsg struct{}

// flashTickMsg redraws while a just-created row's opening beat is lit, the
// same bounded-sequence shape as tabSlideTickMsg.
type flashTickMsg struct{}

type treeMsg struct {
	rows []domain.TreeRow
	err  error
}

var tabs = []string{domain.DashboardTabWorktrees, domain.DashboardTabTree, domain.DashboardTabServices}

// The tab indices; the renderer and the loaders key off them rather than off
// the titles.
const (
	tabWorktrees = iota
	tabTree
	tabServices
)

// Model is the dashboard's root Bubbletea model. It owns its own zone manager
// so hit-testing is per-program state rather than a package global.
type Model struct {
	params     RunParams
	listParams domain.ListParams
	zones      *zone.Manager

	width  int
	height int

	tab      int
	statuses []domain.WorktreeStatus
	parents  map[string]string
	cursor   int
	offset   int
	loaded   bool
	loadErr  error
	loading  bool
	// activeBranch is the worktree the cwd is under, deduced from a path-prefix
	// match — no git call. Empty until a surface computes and sets it.
	activeBranch string
	// repoName names the header's context line and is fixed for the life of the
	// program; the base branch it sits beside is read from config through
	// baseBranch(). fetchedAt dates the last successful fetch (zero when the
	// repository has never fetched) and is refreshed on every reload.
	repoName  string
	fetchedAt time.Time

	prs       []domain.PRInfo
	ghConn    domain.GHConnection
	prsLoaded bool

	// running counts the jobs the run daemon holds per worktree path; jobs is
	// what those counts were derived from, and runConfig what the project
	// declares — the detail panel needs all three.
	running   map[string]int
	jobs      []domain.JobInfo
	runConfig domain.RunConfig
	// addresses is where each worktree's declared jobs answer, keyed
	// branch → job. It follows the poll, like jobs: an address is a property of
	// the worktree's port offset, and two sources for it would diverge.
	addresses map[string]map[string]domain.JobAddress
	// addressNotes is one line per worktree whose .env is unsettled; see
	// addressesMsg.
	addressNotes map[string]string
	// portAddressed follows the addresses, and is handed to the board the logs
	// preview opens; see addressesMsg.
	portAddressed map[string]bool
	// logged names the jobs that left output in each worktree, by branch. With
	// the daemon's index it decides what every surface here shows; see
	// rules.VisibleJobs.
	logged map[string]map[string]bool
	// runExpanded keys the runners whose held addresses are unfolded, by job
	// name. Folded is the default and the state lives here rather than on disk:
	// it is how the panel is being read right now, not a preference — but it
	// outlives moving between worktrees, which is what makes comparing two of
	// them bearable.
	runExpanded map[string]bool
	// board is what the daemon holds up, per worktree. Rebuilt when the jobs,
	// the worktrees or the addresses land — never in the renderer, which asked
	// for it six times a frame with a different time.Now() each time.
	board []rules.RunWorktreeBlock

	outputLines    []string
	outputOffset   int
	outputExpanded bool

	// services is the Services tab flattened into drawn lines, servicesCursor the
	// job row it points at — headers and gaps are drawn, never selected — and
	// servicesOffset the window's first line.
	services       []domain.ServicesRow
	servicesCursor int
	servicesOffset int

	treeRows    []domain.TreeRow
	treeCursor  int
	treeOffset  int
	treeLoaded  bool
	treeLoading bool
	treeErr     error

	detailOpen bool
	showHelp   bool
	// helpScroll is the first visible body row of the reference overlay. It only
	// ever leaves zero on a screen too short to hold the whole reference.
	helpScroll int

	// panelTab is which of the right-hand panel's two tabs is showing.
	panelTab int

	// logsBranch and logsJob name the job whose tail replaces the detail's
	// sections; both empty means the panel is closed.
	logsBranch string
	logsJob    string
	logsLines  []string
	logsErr    error
	// preview is the run view hosted inside the panel: the same renderer and the
	// same live stream `run logs` uses, at a panel's size. It reads no key — the
	// panel owns the navigation, and everything one could act on belongs to the
	// full view, which is what enter opens. previewOn says one is held, since a
	// zero Model is indistinguishable from a live one.
	preview   runview.Model
	previewOn bool

	// details caches the last detail loaded per branch, invalidated (never
	// emptied) by the poll and by a finished operation, so the panel keeps
	// showing "true a second ago" through a reload instead of going blank.
	details       map[string]domain.WorktreeDetail
	detailLoading string
	detailSince   time.Time
	spinner       spinner.Model

	menuOpen   bool
	menuKind   menuKind
	menuCursor int
	// menuAnchor is the cell the context menu hangs from: the click, or the
	// selected row when the keyboard opened it.
	menuAnchor domain.Rect

	// msgs carries what the flow goroutines post; listenCmd is its only reader.
	msgs  chan tea.Msg
	ops   operations
	modal modal
	// selectBranch is the worktree a finished run wants selected once the list
	// catches up with it.
	selectBranch string

	// tabSlideFrom and tabSlideSince drive the tab rule's slide: the column it
	// is animating away from, and when that animation started. A zero
	// tabSlideSince means no slide is in progress — rules.TabSlideStart then
	// reports the target outright.
	tabSlideFrom  int
	tabSlideSince time.Time
	// flashBranch and flashSince drive a just-created row's opening beat: the
	// branch selectRequested last landed the cursor on, and when that
	// happened. A zero flashSince means nothing is flashing.
	flashBranch string
	flashSince  time.Time
}

// New builds the dashboard model. Callers outside a program must Close the
// returned model's zone manager; Run does it for them.
func New(params RunParams) Model {
	return Model{
		params: params,
		listParams: domain.ListParams{
			ProjectDir: params.ProjectDir,
			StateDir:   params.StateDir,
			Config:     params.Config,
		},
		zones:    zone.New(),
		msgs:     make(chan tea.Msg, domain.DashboardMsgBuffer),
		ghConn:   domain.GHConnectionOK,
		loading:  true,
		details:  map[string]domain.WorktreeDetail{},
		spinner:  components.MutedSpinner(),
		repoName: filepath.Base(params.ProjectDir),
	}
}

func (m Model) Close() { m.zones.Close() }

// Run opens the dashboard on the alternate screen. It restores the terminal on
// exit without re-emitting anything into the scrollback.
func Run(params RunParams) error {
	model := New(params)
	defer model.Close()

	if _, err := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run(); err != nil {
		return fmt.Errorf("dashboard: %w", err)
	}
	return nil
}

func (m Model) Init() tea.Cmd {
	// The spinner is started on demand, at the point a detail load actually
	// begins (fireDetailTick, reloadDetailCmd) — not here, or it would tick for
	// the life of the program whether or not anything is loading.
	return tea.Batch(m.loadWorktreesCmd(false), m.loadPRsCmd(), m.loadJobsCmd(true), pollCmd(), gitPollCmd(), listenCmd(m.msgs))
}

func pollCmd() tea.Cmd {
	return tea.Tick(domain.DashboardPollSeconds*time.Second, func(time.Time) tea.Msg { return pollMsg{} })
}

func gitPollCmd() tea.Cmd {
	return tea.Tick(domain.DashboardGitPollSeconds*time.Second, func(time.Time) tea.Msg { return gitPollMsg{} })
}

func tabSlideTickCmd() tea.Cmd {
	return tea.Tick(domain.DashboardAnimFrame, func(time.Time) tea.Msg { return tabSlideTickMsg{} })
}

func flashTickCmd() tea.Cmd {
	return tea.Tick(domain.DashboardAnimFrame, func(time.Time) tea.Msg { return flashTickMsg{} })
}

// loadWorktreesCmd re-lists the worktrees off the UI thread. fetch reaches the
// remote (the `r` key); the poll never does, so the origin badges stay a
// deliberate, user-triggered refresh.
func (m Model) loadWorktreesCmd(fetch bool) tea.Cmd {
	listParams, stateDir, projectDir := m.listParams, m.params.StateDir, m.params.ProjectDir
	return func() tea.Msg {
		list := worktree.List
		if fetch {
			list = worktree.Refresh
		}
		statuses, err := list(listParams)
		if err != nil {
			return worktreesMsg{err: err}
		}
		parents := make(map[string]string, len(statuses))
		for _, status := range statuses {
			parents[status.Branch] = worktree.ParentBranch(worktree.ParentBranchParams{
				StateDir: stateDir,
				Branch:   status.Branch,
			})
		}
		fetchedAt := worktree.LastFetchAt(worktree.LastFetchAtParams{ProjectDir: projectDir})
		return worktreesMsg{statuses: statuses, parents: parents, fetchedAt: fetchedAt}
	}
}

// loadTreeCmd builds the forest off the UI thread. It costs a rev-list per node,
// which is why it is only ever asked for once the Tree tab has been opened.
func (m Model) loadTreeCmd() tea.Cmd {
	listParams, running := m.listParams, m.running
	return func() tea.Msg {
		forest, err := worktree.BuildTree(worktree.BuildTreeParams{
			ProjectDir: listParams.ProjectDir,
			StateDir:   listParams.StateDir,
			Config:     listParams.Config,
		})
		if err != nil {
			return treeMsg{err: err}
		}
		return treeMsg{rows: rules.FlattenForest(rules.ForestWithRunningJobs(forest, running))}
	}
}

// treeCmd asks for a rebuild only when the tab has been opened at least once: a
// user who never looks at the tree never pays for it.
func (m Model) treeCmd() tea.Cmd {
	if !m.treeLoaded && m.tab != tabTree {
		return nil
	}
	return m.loadTreeCmd()
}

func (m Model) loadPRsCmd() tea.Cmd {
	loader := m.params.PRLoader
	if loader == nil {
		return nil
	}
	return func() tea.Msg {
		prs, conn := loader()
		return prsMsg{prs: prs, conn: conn}
	}
}

func (m Model) layout() domain.DashboardLayout {
	return rules.ComputeDashboardLayout(rules.DashboardLayoutParams{
		Width:          m.width,
		Height:         m.height,
		OutputExpanded: m.outputExpanded,
		DetailOpen:     m.detailOpen,
		FullBody:       m.tab == tabServices,
	})
}

// Update answers a message, then sizes the hosted preview to whatever layout
// that left behind. The sizing is done here, once, rather than at each place a
// panel opens or a tab changes: the output panel alone is toggled from a key, a
// click and the start of an operation, and a preview sized at one geometry while
// drawn at another shows the wrong rows.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.update(msg)
	model, ok := next.(Model)
	if !ok {
		return next, cmd
	}
	sized, sizeCmd := model.sizePreview(model.layout())
	return sized, tea.Batch(cmd, sizeCmd)
}

func (m Model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.modal = m.modal.resize(msg.Width, msg.Height)
		return m.reflow(), nil

	case tea.KeyMsg:
		before := m.selectedBranch()
		model, cmd := m.handleKey(msg)
		return withDetailTrigger(before, model, cmd)

	case tea.MouseMsg:
		before := m.selectedBranch()
		model, cmd := m.handleMouse(msg)
		return withDetailTrigger(before, model, cmd)

	case worktreesMsg:
		before := m.selectedBranch()
		next, animCmd := m.applyWorktrees(msg)
		next = next.withBoard()
		model, detailCmd := next.triggerDetailReload(before)
		// An address is not a function of the jobs alone: the loader dials the
		// proxy and reads the worktree's .env, so a proxy that came up late or a
		// port that moved under a job still running is only ever caught here.
		// This is the git clock, and KeyRefresh comes through it too.
		return model, tea.Batch(animCmd, detailCmd, next.resolveAddressesCmd(), next.resolveTracesCmd())

	case tracesMsg:
		m.logged = msg.logged
		return m.withBoard(), nil

	case addressesMsg:
		m.addresses, m.addressNotes, m.portAddressed = msg.addresses, msg.notes, msg.portAddressed
		return m.withBoard(), nil

	case prsMsg:
		m.prs, m.ghConn, m.prsLoaded = msg.prs, msg.conn, true
		return m, nil

	case jobsMsg:
		return m.applyJobs(msg)

	case pollMsg:
		// The logs tail is the detail panel's one exception to being absent from
		// every clock — a tail nobody refreshes is a screenshot.
		return m, tea.Batch(m.loadJobsCmd(false), m.tailLogsCmd(), pollCmd())

	case gitPollMsg:
		if m.loading {
			return m, gitPollCmd()
		}
		m.loading = true
		// The tree only refreshes on the poll while it is on screen; rebuilding a
		// panel nobody is looking at costs a rev-list per node for nothing.
		tree := tea.Cmd(nil)
		if m.tab == tabTree {
			tree = m.loadTreeCmd()
		}
		// The detail panel is deliberately absent from the poll: a reload every
		// few seconds mutes the whole panel behind a "refreshing" marker while
		// the user is reading it. It reloads when the selection changes, when an
		// operation touches its branch, and on KeyRefresh — never on a timer.
		return m, tea.Batch(m.loadWorktreesCmd(false), tree, gitPollCmd())

	case treeMsg:
		before := m.selectedBranch()
		next := m.applyTree(msg)
		return next.triggerDetailReload(before)

	case OutputLineMsg:
		return m.appendOutput(msg), nil

	case openPRMsg:
		if msg.err == nil {
			return m, nil
		}
		return m.appendOutput(OutputLineMsg{
			Text: fmt.Sprintf(domain.DashboardFailedFmt, domain.DashboardOpenPRLabel, msg.err),
		}), nil

	case logsTailMsg:
		return m.applyLogsTail(msg), nil

	case openURLMsg:
		if msg.err == nil {
			return m, nil
		}
		return m.appendOutput(OutputLineMsg{
			Text: fmt.Sprintf(domain.DashboardFailedFmt, domain.DashboardOpenURLLabel, msg.err),
		}), nil

	case flowMsg:
		model, cmd := m.applyFlow(msg.inner)
		return model, tea.Batch(cmd, listenCmd(m.msgs))

	case handoffDoneMsg:
		return m.finishHandoff(msg)

	case opDoneMsg:
		return m.finishOp(msg)

	case detailMsg:
		return m.applyDetail(msg), nil

	case detailTickMsg:
		return m.fireDetailTick(msg)

	case spinner.TickMsg:
		// Re-arming unconditionally would tick at 12fps for the life of the
		// program, idle included. The loop runs while either consumer needs it —
		// a detail load in flight, or a run whose row shows a spinner instead of
		// its state pill — and dies on its own once neither does.
		if m.detailLoading == "" && !m.ops.active() {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tabSlideTickMsg:
		// Bounded: it only re-arms while the slide is still short of its
		// duration, and clears tabSlideSince the moment it settles — an idle
		// dashboard schedules no further ticks once the rule has landed.
		if m.tabSlideSince.IsZero() || time.Since(m.tabSlideSince) >= domain.DashboardTabSlide {
			m.tabSlideSince = time.Time{}
			return m, nil
		}
		return m, tabSlideTickCmd()

	case flashTickMsg:
		// Same bounded shape as tabSlideTickMsg: it stops re-arming, and clears
		// flashBranch, the moment the flash's duration has elapsed.
		if m.flashBranch == "" || time.Since(m.flashSince) >= domain.DashboardRowFlash {
			m.flashBranch = ""
			return m, nil
		}
		return m, flashTickCmd()

	case modalLoadedMsg, formReadyMsg:
		return m.updateModal(msg)
	}

	// Anything left belongs to the hosted preview: its own chunks, its redraw
	// clock, its board refreshes. It is the only sub-model with commands of its
	// own, and it never sees a key or a mouse event — those are handled above,
	// and acting on a job is what the full view is for.
	return m.updatePreview(msg)
}

func (m Model) updatePreview(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !m.previewOn {
		return m, nil
	}
	preview, cmd := m.preview.Update(msg)
	next, ok := preview.(runview.Model)
	if !ok {
		return m, cmd
	}
	m.preview = next
	return m, cmd
}

// withDetailTrigger folds a detail-reload check onto whatever a key or mouse
// event already produced. handleKey and handleMouse return tea.Model because
// they also field the overlays (modal, menu, help), which are not this Model;
// the comma-ok keeps a foreign result harmless instead of panicking.
func withDetailTrigger(before string, next tea.Model, cmd tea.Cmd) (tea.Model, tea.Cmd) {
	model, ok := next.(Model)
	if !ok {
		return next, cmd
	}
	model, detailCmd := model.triggerDetailReload(before)
	return model, tea.Batch(cmd, detailCmd)
}

func (m Model) applyTree(msg treeMsg) Model {
	m.treeLoading, m.treeErr = false, msg.err
	if msg.err != nil {
		return m
	}
	m.treeRows, m.treeLoaded = msg.rows, true
	m.treeCursor = rules.ClampIndex(m.treeCursor, len(m.treeRows))
	return m.reflow()
}

// applyWorktrees keeps the selection on a valid row: a worktree removed under
// the poll must not leave the cursor pointing past the end. It also starts
// the just-created row's opening flash, when selectRequested lands on one and
// ui.animations has not turned that off.
func (m Model) applyWorktrees(msg worktreesMsg) (Model, tea.Cmd) {
	m.loading = false
	m.loadErr = msg.err
	if msg.err != nil {
		return m, nil
	}
	m.statuses, m.parents, m.loaded = msg.statuses, msg.parents, true
	m.fetchedAt = msg.fetchedAt
	m.activeBranch = rules.ActiveWorktree(rules.ActiveWorktreeParams{
		Cwd:      m.params.Cwd,
		Statuses: m.statuses,
	})
	m.cursor = rules.ClampIndex(m.cursor, len(m.statuses))

	animate := rules.AnimationsEnabled(m.params.Config)
	next, flashed := m.selectRequested(animate)
	next = next.reflow()
	if !flashed {
		return next, nil
	}
	return next, flashTickCmd()
}

// selectRequested lands the cursor on the worktree a finished run created, the
// one time the list comes back holding it. animate arms its opening flash,
// gated by ui.animations rather than deciding whether the branch itself
// should flash — a row still gets found and selected either way.
func (m Model) selectRequested(animate bool) (Model, bool) {
	if m.selectBranch == "" {
		return m, false
	}
	for index, status := range m.statuses {
		if status.Branch != m.selectBranch {
			continue
		}
		m.cursor, m.selectBranch = index, ""
		if !animate {
			return m, false
		}
		m.flashBranch, m.flashSince = status.Branch, time.Now()
		return m, true
	}
	return m, false
}

func (m Model) updateModal(msg tea.Msg) (Model, tea.Cmd) {
	modal, cmd := m.modal.update(msg)
	m.modal = modal
	return m, cmd
}

func (m Model) reflow() Model {
	layout := m.layout()
	m.offset = rules.DashboardScrollOffset(rules.DashboardScrollParams{
		Cursor:  m.cursor,
		Total:   len(m.statuses),
		Visible: layout.ListRows,
		Offset:  m.offset,
	})
	m.treeOffset = rules.DashboardScrollOffset(rules.DashboardScrollParams{
		Cursor:  m.treeCursor,
		Total:   len(m.treeRows),
		Visible: layout.TreeRows,
		Offset:  m.treeOffset,
	})
	m.servicesOffset = rules.DashboardScrollOffset(rules.DashboardScrollParams{
		Cursor:  m.servicesCursor,
		Total:   len(m.services),
		Visible: layout.ServicesRows,
		Offset:  m.servicesOffset,
	})
	m.outputOffset = rules.DashboardClampOffset(rules.DashboardOffsetParams{
		Offset:  m.outputOffset,
		Total:   len(m.outputLines),
		Visible: layout.OutputLines,
	})
	return m
}

// selected is the worktree the current tab is pointing at. On the Tree tab a row
// is a node, and a virtual node stands for a branch with no worktree — so it
// selects nothing, and everything keyed off the selection (the detail panel, the
// context menu) correctly finds there is nothing to act on.
func (m Model) selected() (domain.WorktreeStatus, bool) {
	if m.tab == tabTree {
		return m.selectedTreeWorktree()
	}
	if m.tab == tabServices {
		row, ok := m.selectedService()
		if !ok {
			return domain.WorktreeStatus{}, false
		}
		return m.statusFor(row.Branch), true
	}
	if m.cursor < 0 || m.cursor >= len(m.statuses) {
		return domain.WorktreeStatus{}, false
	}
	return m.statuses[m.cursor], true
}

func (m Model) selectedTreeNode() (domain.TreeNode, bool) {
	if m.treeCursor < 0 || m.treeCursor >= len(m.treeRows) {
		return domain.TreeNode{}, false
	}
	return m.treeRows[m.treeCursor].Node, true
}

func (m Model) selectedTreeWorktree() (domain.WorktreeStatus, bool) {
	node, ok := m.selectedTreeNode()
	if !ok || node.IsVirtual {
		return domain.WorktreeStatus{}, false
	}
	for _, status := range m.statuses {
		if status.Branch == node.Branch {
			return status, true
		}
	}
	return domain.WorktreeStatus{}, false
}

func (m Model) moveCursor(delta int) Model {
	if m.tab == tabTree {
		m.treeCursor = rules.ClampIndex(m.treeCursor+delta, len(m.treeRows))
		return m.reflow()
	}
	if m.tab == tabServices {
		return m.stepServices(delta).reflow()
	}
	m.cursor = rules.ClampIndex(m.cursor+delta, len(m.statuses))
	return m.reflow()
}

// rowCount is how many rows the current tab holds, for the keys that jump by a
// page or to an end.
func (m Model) rowCount() int {
	if m.tab == tabTree {
		return len(m.treeRows)
	}
	if m.tab == tabServices {
		return len(m.services)
	}
	return len(m.statuses)
}

func (m Model) scrollOutput(delta int) Model {
	m.outputOffset = max(m.outputOffset+delta, 0)
	return m.reflow()
}

func (m Model) refresh() (Model, tea.Cmd) {
	m.loading, m.prsLoaded = true, false
	m.treeLoading = m.treeLoaded || m.tab == tabTree
	next, detailCmd := m.reloadDetailCmd()
	return next, tea.Batch(next.loadWorktreesCmd(true), next.loadPRsCmd(), next.loadJobsCmd(true), next.treeCmd(), detailCmd)
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// A modal owns the keyboard while it is up: it is a question, and nothing
	// behind it may be acted on before it is answered.
	if m.modal.open {
		if key == keyInterrupt {
			return m, tea.Quit
		}
		return m.updateModal(msg)
	}

	if m.showHelp {
		return m.helpKey(key)
	}

	// An open menu takes the keys it uses and lets every other one through, after
	// closing itself: a dropdown must never trap the keyboard.
	if m.menuOpen {
		switch key {
		case keyUp, keyVimUp:
			return m.moveMenu(-1), nil
		case keyDown, keyVimDown:
			return m.moveMenu(1), nil
		case keyEnter:
			return m.activateMenu(m.menuCursor)
		case keyEscape, keyMenu:
			return m.closeMenu(), nil
		}
		m = m.closeMenu()
	}

	layout := m.layout()

	// The logs panel owns esc and enter while it is up: esc gives the detail
	// back, enter hands the terminal to runview for the session this is only a
	// glance at.
	if m.logsOpen() {
		switch key {
		case keyEscape:
			return m.closePanelLogs().reflow(), nil
		// The jobs list down the side, so up and down walk it. The arrows it
		// answered when it was a row still work: a reader who learnt them there
		// does not have to learn them again.
		case keyUp, keyVimUp, keyLeft, keyVimLeft:
			return m.stepLogsJob(-1).retailAndPreview()
		case keyDown, keyVimDown, keyRight, keyVimRight:
			return m.stepLogsJob(1).retailAndPreview()
		case keyEnter:
			return m.watchLogs()
		}
	}

	switch key {
	case keyInterrupt, keyQuit:
		return m, tea.Quit
	case keyHelp:
		m.showHelp, m.helpScroll = true, 0
	case keyRefresh:
		return m.refresh()
	case keyNew:
		return m.startCreate()
	case keyMenu:
		return m.openMenu(m.menuAnchorPoint()), nil
	case keyActions:
		return m.openActionsMenu(m.actionsAnchorPoint()), nil
	case keyOpenPR:
		return m.openPR()
	case keyRunLogs:
		return m.openLogsTab()
	case keyOpenAddress:
		return m.openSelectedAddress()
	case keyFastForward:
		selected, ok := m.selected()
		if !ok {
			return m, nil
		}
		return m.startFastForward(selected.Branch)

	case keyToggleOutput:
		m.outputExpanded = !m.outputExpanded
		return m.reflow(), nil
	case keyTab:
		return m.selectTab((m.tab + 1) % len(tabs))
	case keyShiftTab:
		return m.selectTab((m.tab + len(tabs) - 1) % len(tabs))
	case keyUp, keyVimUp:
		return m.moveCursor(-1), nil
	case keyDown, keyVimDown:
		return m.moveCursor(1), nil
	case keyPageUp:
		return m.moveCursor(-max(m.pageRows(layout), 1)), nil
	case keyPageDown:
		return m.moveCursor(max(m.pageRows(layout), 1)), nil
	case keyTop:
		return m.moveCursor(-m.rowCount()), nil
	case keyBottom:
		return m.moveCursor(m.rowCount()), nil
	case keyOutputUp:
		return m.scrollOutput(-1), nil
	case keyOutputDown:
		return m.scrollOutput(1), nil
	case keyEnter, keyRight, keyVimRight:
		if m.tab == tabServices && key == keyEnter {
			return m.watchServiceLogs()
		}
		if layout.Narrow {
			m.detailOpen = true
			return m.reflow(), nil
		}
	case keyEscape, keyLeft, keyVimLeft:
		if m.detailOpen {
			m.detailOpen = false
			return m.reflow(), nil
		}
	}

	return m, nil
}

// selectTab moves to a tab and, the first time the Tree tab is opened, asks for
// the forest it has never built. It also starts the tab rule's slide toward
// its new position, when ui.animations has not turned that off and the rule
// actually moves — switching to the tab already active is a no-op either way.
func (m Model) selectTab(index int) (Model, tea.Cmd) {
	// The logs view belongs to the tab it is drawn in: left open across a tab
	// change it kept esc and enter while showing nothing.
	m = m.closePanelLogs()
	width := m.layout().Tabs.Width
	from := tabStart(width, m.tab)
	m.tab = index

	var slideCmd tea.Cmd
	if rules.AnimationsEnabled(m.params.Config) {
		if to := tabStart(width, index); to != from {
			m.tabSlideFrom, m.tabSlideSince = from, time.Now()
			slideCmd = tabSlideTickCmd()
		}
	}

	if index != tabTree || m.treeLoaded || m.treeLoading {
		return m.reflow(), slideCmd
	}
	m.treeLoading = true
	return m.reflow(), tea.Batch(slideCmd, m.loadTreeCmd())
}

func (m Model) pageRows(layout domain.DashboardLayout) int {
	switch m.tab {
	case tabTree:
		return layout.TreeRows
	case tabServices:
		return layout.ServicesRows
	}
	return layout.ListRows
}

func (m Model) inZone(id string, msg tea.MouseMsg) bool {
	return m.zones.Get(id).InBounds(msg)
}

func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.showHelp {
		return m.helpMouse(msg)
	}
	if m.modal.open {
		return m.modalMouse(msg)
	}
	if m.menuOpen {
		return m.menuMouse(msg)
	}

	switch msg.Button {
	case tea.MouseButtonWheelUp:
		return m.wheel(msg, -1), nil
	case tea.MouseButtonWheelDown:
		return m.wheel(msg, 1), nil
	}

	if msg.Action != tea.MouseActionPress {
		return m, nil
	}
	if msg.Button == tea.MouseButtonRight {
		return m.rightClick(msg)
	}
	if msg.Button != tea.MouseButtonLeft {
		return m, nil
	}

	for index := range tabs {
		if m.inZone(tabZone(index), msg) {
			return m.selectTab(index)
		}
	}

	if m.inZone(zoneOutputToggle, msg) {
		m.outputExpanded = !m.outputExpanded
		return m.reflow(), nil
	}

	if m.inZone(zoneAdd, msg) {
		return m.startCreate()
	}

	if m.inZone(zoneActions, msg) {
		zone := m.zones.Get(zoneActions)
		return m.openActionsMenu(domain.Rect{X: zone.StartX, Y: zone.EndY}), nil
	}

	if m.inZone(zonePanelTabDtl, msg) {
		return m.closePanelLogs().reflow(), nil
	}
	if m.inZone(zonePanelTabLogs, msg) {
		return m.openLogsTab()
	}

	if m.inZone(zoneDetailPR, msg) {
		return m.openPR()
	}

	if model, cmd, hit := m.clickLogsAddress(msg); hit {
		return model, cmd
	}

	if model, cmd, hit := m.clickLogsJob(msg); hit {
		return model, cmd
	}

	if model, cmd, hit := m.clickRunRow(msg); hit {
		return model, cmd
	}

	if model, cmd, hit := m.clickServiceRow(msg); hit {
		return model, cmd
	}

	if model, hit := m.clickRow(msg); hit {
		return model, nil
	}

	return m, nil
}

// clickRow selects the row under the pointer on whichever tab is showing.
func (m Model) clickRow(msg tea.MouseMsg) (Model, bool) {
	if m.tab == tabServices {
		// clickServiceRow owns those rows: it selects and then opens, which a
		// bare selection here would pre-empt.
		return m, false
	}
	if m.tab == tabTree {
		for index := range m.treeRows {
			if !m.inZone(treeRowZone(index), msg) {
				continue
			}
			m.treeCursor = index
			return m.reflow(), true
		}
		return m, false
	}
	for index := range m.statuses {
		if !m.inZone(rowZone(index), msg) {
			continue
		}
		m.cursor = index
		if m.layout().Narrow {
			m.detailOpen = true
		}
		return m.reflow(), true
	}
	return m, false
}

// menuMouse gives the menu the mouse while it is up: an entry activates, and
// anything else — a click beside it, the wheel, the right button again —
// dismisses it and does nothing more, as a context menu does everywhere.
func (m Model) menuMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Action != tea.MouseActionPress {
		return m, nil
	}
	if msg.Button == tea.MouseButtonLeft {
		for index := range m.menuItems() {
			if m.inZone(menuZone(index), msg) {
				return m.activateMenu(index)
			}
		}
	}
	return m.closeMenu(), nil
}

// rightClick opens the context menu on the row it lands on, selecting it first:
// a menu that acted on another row than the one under the pointer would be a trap.
func (m Model) rightClick(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	model, hit := m.selectRow(msg)
	if !hit {
		return m, nil
	}
	return model.openMenu(domain.Rect{X: msg.X, Y: msg.Y}), nil
}

// selectRow puts the cursor on the row under the pointer, whichever tab is
// showing. The Services tab reaches it here rather than through clickRow, which
// leaves those rows to clickServiceRow: the left button selects and then opens,
// and a bare selection there would pre-empt the opening.
func (m Model) selectRow(msg tea.MouseMsg) (Model, bool) {
	if m.tab == tabServices {
		return m.selectServiceRow(msg)
	}
	return m.clickRow(msg)
}

// modalMouse only ever resolves the modal's own rows: the frame behind it is
// still on the zone manager from the last frame that drew it.
func (m Model) modalMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}
	for index := range m.modal.rows {
		if !m.inZone(modalRowZone(index), msg) {
			continue
		}
		modal, cmd := m.modal.activate(index)
		m.modal = modal
		return m, cmd
	}
	return m, nil
}

func (m Model) wheel(msg tea.MouseMsg, delta int) Model {
	if m.outputExpanded && m.inZone(zoneOutput, msg) {
		return m.scrollOutput(delta)
	}
	if m.inZone(zoneList, msg) || m.inZone(zoneTree, msg) || m.inZone(zoneServices, msg) {
		return m.moveCursor(delta)
	}
	return m
}

func (m Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}

	layout := m.layout()
	body := ""
	switch {
	case layout.DetailVisible && layout.ListVisible:
		body = lipgloss.JoinHorizontal(lipgloss.Top, m.renderMain(layout), m.renderDetail(layout))
	case layout.DetailVisible:
		body = m.renderDetail(layout)
	default:
		body = m.renderMain(layout)
	}

	// A panel too small to draw returns nothing; keeping its empty line would push
	// the frame past the last row and make the alt screen scroll.
	sections := make([]string, 0, 4)
	for _, section := range []string{m.renderHeader(layout), body, m.renderOutput(layout), m.renderHelpBar(layout)} {
		if section != "" {
			sections = append(sections, section)
		}
	}

	return m.zones.Scan(m.withOverlays(lipgloss.JoinVertical(lipgloss.Left, sections...)))
}

// renderMain draws whichever tab owns the main panel. The Tree tab takes the
// list's place rather than the whole body, so the detail stays beside it and a
// node keeps leading somewhere.
func (m Model) renderMain(layout domain.DashboardLayout) string {
	switch m.tab {
	case tabTree:
		return m.renderTree(layout)
	case tabServices:
		return m.renderServices(layout)
	}
	return m.renderList(layout)
}

// withOverlays pastes whatever is open over the frame. Only one ever is: each of
// them takes the keyboard while it is up.
func (m Model) withOverlays(frame string) string {
	if m.showHelp {
		box, rect := m.helpBox()
		return overlay(overlayParams{Base: frame, Box: box, At: rect})
	}
	if m.modal.open {
		box, rect := m.modal.box(m.zones)
		return overlay(overlayParams{Base: frame, Box: box, At: rect})
	}
	if m.menuOpen {
		box, rect := m.menuBox()
		return overlay(overlayParams{Base: frame, Box: box, At: rect})
	}
	return frame
}

// loadJobsCmd reads the run daemon's index off the UI thread, with the run.toml
// those jobs are declared in. It is graceful by design: no daemon means nothing
// is running, which is an answer and not an error, so it never reaches the
// output panel.
func (m Model) loadJobsCmd(wake bool) tea.Cmd {
	load, stateDir := m.jobsLoader(), m.params.StateDir
	return func() tea.Msg {
		jobs, known := load(wake)
		cfg, _ := runconfig.Load(stateDir)
		return jobsMsg{jobs: jobs, running: rules.RunningJobsByWorktree(jobs), config: cfg, known: known}
	}
}

// withBoard rebuilds what the daemon holds up and the lines the Services tab
// draws from it, then re-seats that tab's cursor and offset on the new list.
func (m Model) withBoard() Model {
	m.board = rules.RunBoard(rules.RunBoardParams{
		Config:    m.runConfig,
		Jobs:      m.jobs,
		Addresses: m.addresses,
		Notes:     m.addressNotes,
		Expanded:  m.runExpanded,
		Statuses:  m.statuses,
		Now:       time.Now(),
	})
	m.services = rules.ServicesRows(m.board)
	// Re-bound here, not only when an arrow is pressed: a job stopping shrinks
	// the list under a cursor nobody moved, and selected() then answered
	// "nothing" — no menu, no selection, until the user pressed a key.
	m.servicesCursor = m.nearestServiceJob(nearestJobParams{Index: m.servicesCursor, Direction: 1})
	// The offset with it: a board that shrinks under an offset nobody moved
	// leaves servicesVisible past the end, and the tab draws nothing at all.
	return m.reflow()
}

// resolveTracesCmd asks what each worktree has left on disk. Unlike the
// addresses it covers every worktree the list holds, running or not: a trace is
// what a job leaves once the daemon has dropped it, so the worktrees with
// nothing up are exactly the ones with something to report.
func (m Model) resolveTracesCmd() tea.Cmd {
	if m.params.TraceLoader == nil || len(m.runConfig.Jobs) == 0 || len(m.statuses) == 0 {
		return nil
	}
	branches := make([]string, 0, len(m.statuses))
	for _, status := range m.statuses {
		branches = append(branches, status.Branch)
	}
	load := m.params.TraceLoader
	return func() tea.Msg { return tracesMsg{logged: load(branches)} }
}

// resolveAddressesCmd asks where the running worktrees' jobs answer. It is
// built from the model the jobs have already been applied to, never captured by
// the command that read them: Init loads the worktrees and the jobs in
// parallel, so that model's statuses may still be empty — which asked for no
// address at all and left the RUN section without one until the next poll.
//
// Only the worktrees that already have a job up are named: BranchEnv allocates
// an ordinal to whichever branch it is handed, and an idle worktree must not be
// given one just because a poll swept past it.
func (m Model) resolveAddressesCmd() tea.Cmd {
	if m.params.AddressLoader == nil || len(m.runConfig.Jobs) == 0 {
		return nil
	}
	branches := rules.BranchesWithJobsUp(rules.BranchesWithJobsUpParams{Jobs: m.jobs, Statuses: m.statuses})
	if len(branches) == 0 {
		return nil
	}
	load, request := m.params.AddressLoader, AddressRequest{Branches: branches, Config: m.runConfig}
	return func() tea.Msg {
		answer := load(request)
		return addressesMsg{addresses: answer.ByBranch, notes: answer.Notes, portAddressed: answer.PortAddressed}
	}
}

func (m Model) jobsLoader() func(bool) ([]domain.JobInfo, bool) {
	if m.params.JobsLoader != nil {
		return m.params.JobsLoader
	}
	return defaultJobsLoader
}

// A waking read always knows: it opens the daemon rather than asking whether
// one happens to be listening.
func defaultJobsLoader(wake bool) ([]domain.JobInfo, bool) {
	if wake {
		return runjobs.Load(), true
	}
	return runjobs.Peek()
}

// applyJobs also reloads the detail on screen when the project's declared jobs
// changed: a panel built before run.toml was read would otherwise stay without
// its RUN section for the rest of the session.
func (m Model) applyJobs(msg jobsMsg) (Model, tea.Cmd) {
	changed := !rules.SameRunJobs(m.runConfig, msg.config)
	m.runConfig = msg.config
	// What the ordinals, the traces and the tree's per-node counts are derived
	// from is the running set, counts included: a job stopping beside another
	// still up moves no branch in or out, and the tree would carry the old count
	// until the git clock came round. A poll that finds it unmoved — the
	// overwhelming majority of them — re-derives nothing.
	moved := changed
	if msg.known {
		moved = moved || !maps.Equal(m.running, msg.running)
		m.jobs, m.running = msg.jobs, msg.running
	}
	// The tree carries the count on its nodes, so the rows already drawn hold a
	// stale one until they are rebuilt.
	m = m.withBoard()
	if !moved {
		return m, nil
	}
	if !changed {
		return m, tea.Batch(m.treeCmd(), m.resolveAddressesCmd(), m.resolveTracesCmd())
	}
	next, detailCmd := m.invalidateDetail(m.selectedBranch())
	return next, tea.Batch(next.treeCmd(), detailCmd, next.resolveAddressesCmd(), next.resolveTracesCmd())
}
