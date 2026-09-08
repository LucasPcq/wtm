package dashboard

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/LucasPcq/wtm/internal/domain"
	logsflow "github.com/LucasPcq/wtm/internal/flow/run/logs"
	"github.com/LucasPcq/wtm/internal/flow/run/seam"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
	"github.com/LucasPcq/wtm/internal/tui/runview"
)

// logsRequest is what a tail needs and the dashboard has to supply: the daemon
// keys a job on its name and the worktree it runs in.
type logsRequest struct {
	WorkDir string
	Job     string
	Jobs    []domain.JobConfig
	Lines   int
	// PortAddressed says this worktree's .env still answers on ports, which is
	// the one thing the board cannot read for itself. Without it the preview
	// announced ports under a header announcing the url — the same job's address
	// spelled two ways, two lines apart.
	PortAddressed bool
}

type logsTailMsg struct {
	branch string
	job    string
	lines  []string
	err    error
}

type LogsLoaderParams struct {
	ProjectDir string
	StateDir   string
	// PublicPort is where a published name answers. It is a func because it is
	// dialed, and dialing belongs off the goroutine that draws: the daemon may
	// have been started after the dashboard was opened, and a port read once at
	// startup would leave every preview address unpublished for the session.
	PublicPort func() int
}

// DefaultBoardLoader opens the worktree's board, which is what a live preview
// attaches through. NoProbe: nothing is being started here, so there is no port
// to wait for.
func DefaultBoardLoader(params LogsLoaderParams) func(logsRequest) runlogs.Board {
	return func(req logsRequest) runlogs.Board {
		return seam.Open(seam.Params{
			ProjectDir:    params.ProjectDir,
			StateDir:      params.StateDir,
			WorkDir:       req.WorkDir,
			Jobs:          req.Jobs,
			PublicPort:    publicPortOf(params),
			PortAddressed: req.PortAddressed,
			NoProbe:       true,
		}).Board()
	}
}

// publicPortOf is zero for a surface that was given no dialer — a test, or a
// dashboard opened with no run module.
func publicPortOf(params LogsLoaderParams) int {
	if params.PublicPort == nil {
		return 0
	}
	return params.PublicPort()
}

// DefaultLogsLoader reads back what a job persisted, whether or not it still
// runs. NoProbe: nothing is being started here, so there is no port to wait for.
func DefaultLogsLoader(params LogsLoaderParams) func(logsRequest) ([]string, error) {
	return func(req logsRequest) ([]string, error) {
		board := seam.Open(seam.Params{
			ProjectDir: params.ProjectDir,
			StateDir:   params.StateDir,
			WorkDir:    req.WorkDir,
			Jobs:       req.Jobs,
			NoProbe:    true,
		}).Board()
		// No Refresh: History tails the job's log file, and asking the daemon
		// first made a dead daemon look like an unreadable log — in the very
		// case one opens the logs for.
		return board.History(runlogs.HistoryParams{Job: req.Job, Lines: req.Lines})
	}
}

// openLogsTab shows the panel's logs view. It lands on the first job that is
// up, or on the first declared one when nothing is: opening on a job rather
// than on a question is what removes the picker.
// openLogsTab shows the logs view in whichever host is on screen. On the
// Services tab that is its own body: arming the right-hand panel there left an
// invisible view holding esc, enter and the arrows.
func (m Model) openLogsTab() (Model, tea.Cmd) {
	// From the Services tab the panel is not drawn at all: arming it there left
	// an invisible view holding esc, enter and the arrows. The tab opens the full
	// run view on the job under its cursor instead.
	if m.tab == tabServices {
		return m.watchServiceLogs()
	}
	return m.openLogsTabOn("")
}

// openLogsTabOn is the same, on a job the surface already designates — a click
// on its row.
func (m Model) openLogsTabOn(job string) (Model, tea.Cmd) {
	status, ok := m.selected()
	if !ok {
		return m, nil
	}
	m.panelTab = panelLogs
	m.logsBranch = status.Branch
	m.logsJob = job
	if job == "" {
		m.logsJob = m.firstLogsJob()
	}
	m.logsLines, m.logsErr = nil, nil
	model, previewCmd := m.openPreview()
	return model, tea.Batch(model.tailLogsCmd(), previewCmd)
}

// firstLogsJob is where the view opens: what is up, else the first declared
// job, else nothing — a project with no run module still opens the tab, which
// then says what would fill it.
func (m Model) firstLogsJob() string {
	jobs := m.logsJobs()
	if len(jobs) == 0 {
		return ""
	}
	for _, job := range jobs {
		if m.jobIsUp(job.Name) {
			return job.Name
		}
	}
	return jobs[0].Name
}

// closePanelLogs and closeServiceLogs each put away their own host. The tail
// itself is shared, so it is only dropped once neither host shows it: clearing
// it from one closed the other's view out from under it.
func (m Model) closePanelLogs() Model {
	m.panelTab = panelDetail
	return m.forgetHiddenLogs()
}

// openPreview holds a run view at the panel's size, on the job the panel is
// showing. Without a BoardLoader — a test, a surface with no daemon — the panel
// keeps its persisted tail and nothing else changes.
func (m Model) openPreview() (Model, tea.Cmd) {
	if m.params.BoardLoader == nil || !m.logsOpen() || m.logsJob == "" {
		return m, nil
	}
	m = m.closePreview()

	board := m.params.BoardLoader(logsRequest{
		WorkDir:       m.statusFor(m.logsBranch).Path,
		Jobs:          m.runConfig.Jobs,
		PortAddressed: m.portAddressed[m.logsBranch],
	})
	if board == nil {
		return m, nil
	}

	m.preview = runview.NewPreview(runview.PreviewParams{Board: board, Job: m.logsJob})
	m.previewOn = true

	// Init is what asks the board for its jobs and opens the stream: a preview
	// whose commands are never run shows an empty pane for ever.
	model, sizeCmd := m.sizePreview(m.layout())
	return model, tea.Batch(model.preview.Init(), sizeCmd)
}

// closePreview releases the stream the panel was holding. A panel closed
// without it leaks one subscription per job it ever showed.
func (m Model) closePreview() Model {
	if !m.previewOn {
		return m
	}
	m.preview.Close()
	m.preview, m.previewOn = runview.Model{}, false
	return m
}

// showPreviewJob follows the panel's own cursor. The preview reads no key: the
// panel says which job, and everything one could act on belongs to the full
// view.
func (m Model) showPreviewJob() (Model, tea.Cmd) {
	if !m.previewOn {
		return m, nil
	}
	preview, cmd := m.preview.ShowJob(m.logsJob)
	m.preview = preview
	return m, cmd
}

// sizePreview gives the preview exactly the rows and columns the panel draws
// it into.
func (m Model) sizePreview(layout domain.DashboardLayout) (Model, tea.Cmd) {
	if !m.previewOn {
		return m, nil
	}
	tail := m.logsTailRect(m.logsHostRect(layout))
	preview, cmd := m.preview.SetSize(tail.Width, tail.Height)
	m.preview = preview
	return m, cmd
}

// logsHostRect is the room the logs view has: the right-hand panel, under its
// LOGS tab.
func (m Model) logsHostRect(layout domain.DashboardLayout) logsViewParams {
	return logsViewParams{
		Width:  layout.Detail.Width - borderWidth - paddingWidth,
		Height: tabbedPanelBodyHeight(layout.Detail),
	}
}

func (m Model) forgetHiddenLogs() Model {
	if m.logsOpen() {
		return m
	}
	m = m.closePreview()
	m.logsBranch, m.logsJob = "", ""
	m.logsLines, m.logsErr = nil, nil
	return m
}

// logsOpen is true wherever the logs view is drawn: under the panel's LOGS tab,
// or as the Services tab's body. The keys it owns follow the view, not a host,
// and it is open even with no job to show — esc has to get back out of an
// empty view too.
func (m Model) logsOpen() bool { return m.panelTab == panelLogs }

// retailAndPreview follows a job change on both readings: the persisted tail
// the panel falls back to, and the live preview when one is held.
func (m Model) retailAndPreview() (Model, tea.Cmd) {
	model, previewCmd := m.showPreviewJob()
	return model, tea.Batch(model.tailLogsCmd(), previewCmd)
}

// watchLogsRequest is what enter hands runview: the job on screen, never the
// whole worktree. A view opened on one job that comes back showing every job of
// the worktree is not the same view.
func (m Model) watchLogsRequest() logsflow.Request {
	return logsflow.Request{
		Worktrees: []string{m.logsBranch},
		Cwd:       m.statusFor(m.logsBranch).Path,
		Job:       m.logsJob,
	}
}

func (m Model) tailLogsCmd() tea.Cmd {
	// A live preview reads the same output from the daemon: re-reading the log
	// file behind it would be a disk read per job change for lines nothing shows.
	if m.params.LogsLoader == nil || !m.logsOpen() || m.previewOn {
		return nil
	}
	load, branch, job := m.params.LogsLoader, m.logsBranch, m.logsJob
	req := logsRequest{
		WorkDir: m.statusFor(branch).Path,
		Job:     job,
		Jobs:    m.runConfig.Jobs,
		Lines:   domain.DashboardLogsLines,
	}
	return func() tea.Msg {
		lines, err := load(req)
		return logsTailMsg{branch: branch, job: job, lines: lines, err: err}
	}
}

// A superseded tail landing late must not overwrite the panel now open — the
// same rule applyDetail follows.
func (m Model) applyLogsTail(msg logsTailMsg) Model {
	if msg.branch != m.logsBranch || msg.job != m.logsJob {
		return m
	}
	if msg.err != nil {
		m.logsErr = msg.err
		return m
	}
	m.logsLines, m.logsErr = msg.lines, nil
	return m
}

// clickLogsAddress answers a click on one of the addresses the logs view heads
// with — the job's own, or one of those a runner answers for.
func (m Model) clickLogsAddress(msg tea.MouseMsg) (tea.Model, tea.Cmd, bool) {
	if !m.logsOpen() {
		return m, nil, false
	}
	address := m.logsAddress()
	for _, held := range address.Held {
		if m.inZone(logsHeldZone(held.Job), msg) {
			model, cmd := m.openJobURL(held.URL)
			return model, cmd, true
		}
	}
	if !m.inZone(logsURLZone(), msg) {
		return m, nil, false
	}
	model, cmd := m.openJobURL(address.URL)
	return model, cmd, true
}

// clickLogsJob answers a click on the selection line. It serves both hosts:
// the line is the same component wherever it is drawn.
func (m Model) clickLogsJob(msg tea.MouseMsg) (tea.Model, tea.Cmd, bool) {
	if !m.logsOpen() {
		return m, nil, false
	}
	for _, job := range m.logsJobs() {
		if !m.inZone(logsJobZone(job.Name), msg) {
			continue
		}
		if job.Name == m.logsJob {
			return m, nil, true
		}
		m.logsJob, m.logsLines, m.logsErr = job.Name, nil, nil
		return m, m.tailLogsCmd(), true
	}
	return m, nil, false
}

// logsJobs are the jobs the column offers, read by the very rule the RUN
// section reads: what lives or left a trace in this worktree, in run.toml's
// order. Offering every declaration put fifteen names in front of a reader of
// whom twelve answered "No output recorded for this job." — the column proposed
// what it had no way to show.
//
// A stopped or crashed job keeps its place, which is the whole point: History
// reads back what it persisted, and that is exactly what one comes here for.
func (m Model) logsJobs() []domain.JobConfig {
	visible, _ := m.visibleLogsJobs()
	jobs := make([]domain.JobConfig, 0, len(visible))
	for _, job := range visible {
		jobs = append(jobs, job.Job)
	}
	return jobs
}

// logsDeclaredMore is what run.toml holds beyond the column, for the line that
// closes it — the same catalogue the RUN section closes with.
func (m Model) logsDeclaredMore() int {
	_, hidden := m.visibleLogsJobs()
	return hidden
}

func (m Model) visibleLogsJobs() ([]rules.VisibleJob, int) {
	status := m.statusFor(m.logsBranch)
	return rules.VisibleJobs(rules.VisibleJobsParams{
		Jobs: m.runConfig.Jobs,
		Up:   rules.IndexedJobsByName(m.jobs, status.Path),
		// The logs view is the one surface whose subject is the archive: `run up`
		// clears the logs of what it is not starting, so what is left on disk is
		// this run — including the jobs of it that have already finished.
		Traces: m.logged[m.logsBranch],
	})
}

// logsAddressLines head the logs view with where the job on screen answers. A
// job publishing one name gets one line; a runner, which publishes nothing of
// its own and answers for its children, gets one line per address it holds.
// They belong here and not in the job column beside them: this row has always
// been where an address is read, and the column is for picking a log.
//
// Each line is its own zone, so each address is a thing to click — the count
// they used to share named something it gave no way to reach.
func (m Model) logsAddressLines(width int) []string {
	address := m.logsAddress()
	if len(address.Held) > 0 {
		lines := make([]string, 0, len(address.Held))
		for index, text := range rules.HeldAddressLines(address.Held) {
			lines = append(lines, m.marks().Mark(logsHeldZone(address.Held[index].Job),
				styles.DashboardURL.Render(truncate(text, width))))
		}
		return lines
	}

	if text := rules.JobAddressText(address); text != "" {
		// Only a url is a zone: a click has to lead somewhere, and a list of ports
		// leads nowhere.
		if address.URL == "" {
			return []string{styles.DashboardRowMeta.Render(truncate(text, width))}
		}
		return []string{m.marks().Mark(logsURLZone(), styles.DashboardURL.Render(truncate(text, width)))}
	}

	// A job that declares no port has no address to show, and the row is kept
	// anyway to hold the body still. Saying so beats a hole: it is the same
	// finding `run init` reports, at the moment one wonders where to reach the
	// job.
	if m.logsJob == "" {
		return []string{""}
	}
	return []string{styles.DashboardRowMeta.Render(truncate(domain.DashboardLogsNoAddress, width))}
}

// logsHeadRows is what the head costs the body. It is read by the renderer and
// by the sizer alike: an emulator fed at one height and drawn at another shows
// the wrong rows.
func (m Model) logsHeadRows() int {
	if held := len(m.logsAddress().Held); held > 0 {
		return held + domain.DashboardLogsHead - 1
	}
	return domain.DashboardLogsHead
}

type logsJobColumnParams struct {
	Width int
	Rows  int
}

// logsJobColumn lists the jobs down the side of their output. It was a row of
// chips windowed to the width, which on a worktree with ten jobs hid most of
// them behind ‹ › marks and made reaching one a traversal. A column scrolls the
// way every other list in the dashboard does.
func (m Model) logsJobColumn(params logsJobColumnParams) []string {
	jobs := m.logsJobs()
	if len(jobs) == 0 || params.Width <= 0 || params.Rows <= 0 {
		return nil
	}

	// A live preview spends its first row on the pane's border and its second on
	// the job's name, so the column starts one row down and its highlighted job
	// sits level with the job the pane names. The persisted tail has no border,
	// and starts level with its first line.
	offset := 0
	if m.previewOn {
		offset = domain.DashboardLogsColumnOffset
	}
	listRows := max(params.Rows-offset, 0)
	window := rules.LogsJobColumn(rules.LogsJobColumnParams{
		Jobs:    jobNames(jobs),
		Current: m.logsJob,
		Rows:    listRows,
	})

	rows := make([]string, 0, params.Rows)
	for range offset {
		rows = append(rows, strings.Repeat(" ", params.Width))
	}
	for _, job := range jobs[window.Start:window.End] {
		chip := truncate(m.logsJobChip(job), params.Width)
		rows = append(rows, m.marks().Mark(logsJobZone(job.Name), pad(chip, params.Width)))
	}
	// The catalogue closes the column as it closes the RUN section, and only
	// when the whole list is on screen: a line saying what run.toml holds beyond
	// a window that is itself scrolled would be counting two different things.
	if more := m.logsDeclaredMore(); more > 0 && window.End == len(jobs) && len(rows) < params.Rows {
		note := truncate(styles.DashboardRowMeta.Render(fmt.Sprintf(domain.DetailDeclaredMoreFmt, more)), params.Width)
		rows = append(rows, pad(note, params.Width))
	}
	for len(rows) < params.Rows {
		rows = append(rows, strings.Repeat(" ", params.Width))
	}
	return rows
}

func jobNames(jobs []domain.JobConfig) []string {
	names := make([]string, 0, len(jobs))
	for _, job := range jobs {
		names = append(names, job.Name)
	}
	return names
}

// The column is a job selector and nothing else. A runner's addresses were
// listed under it for a while: on a twenty-column list that doubled its length,
// buried the jobs, and mixed two gestures — picking a log to read, and reaching
// a url. They head the view instead, in the row that has always been where an
// address is read.
func (m Model) logsJobChip(job domain.JobConfig) string {
	visible, _ := m.visibleLogsJobs()
	state := rules.JobStateStopped
	for _, entry := range visible {
		if entry.Job.Name == job.Name {
			state = entry.State
		}
	}

	glyph, style := rules.JobStateGlyph(state), styles.DashboardRowMeta
	if m.jobIsUp(job.Name) {
		style = styles.DashboardValue
	}
	if job.Name == m.logsJob {
		style = styles.DashboardRowSelected
	}
	return style.Render(glyph + domain.DetailGlyphGap + job.Name)
}

func (m Model) jobIsUp(name string) bool {
	workDir := m.statusFor(m.logsBranch).Path
	for _, info := range m.jobs {
		if info.Name == name && info.WorkDir == workDir && rules.IsJobUp(info.Status) {
			return true
		}
	}
	return false
}

// logsAddress is where the job on screen answers — read off logsBranch, never
// off the selected worktree: the two part company as soon as the logs view is
// hosted by the Services tab.
func (m Model) logsAddress() domain.JobAddress {
	return m.addresses[m.logsBranch][m.logsJob]
}

// stepLogsJob walks the job column, clamped at both ends: it is a list, not a
// carousel, so an end that wraps would lose the reader.
func (m Model) stepLogsJob(delta int) Model {
	jobs := m.logsJobs()
	if len(jobs) == 0 {
		return m
	}
	// A run.toml edited under the view can drop the job on screen. Landing on
	// the first one would make an arrow jump somewhere unrelated, so an unknown
	// job is re-seated before it is stepped from.
	index, known := logsJobIndex(jobs, m.logsJob)
	if !known {
		m.logsJob, m.logsLines, m.logsErr = jobs[0].Name, nil, nil
		return m
	}
	next := jobs[rules.ClampIndex(index+delta, len(jobs))]
	if next.Name == m.logsJob {
		return m
	}
	m.logsJob, m.logsLines, m.logsErr = next.Name, nil, nil
	return m
}

type logsViewParams struct {
	Width  int
	Height int
}

// logsTailRect is the room left to a job's output once the address, the hint
// and the job column have taken theirs. The renderer draws into it and the
// preview is sized to it: an emulator fed at one size and drawn at another
// shows the wrong rows.
func (m Model) logsTailRect(params logsViewParams) logsViewParams {
	gap := lipgloss.Width(domain.DashboardLogsJobGap)
	colWidth := rules.LogsJobColumnWidth(rules.LogsJobColumnWidthParams{
		Names:   jobNames(m.logsJobs()),
		Total:   params.Width,
		Glyph:   lipgloss.Width(domain.DetailJobUpGlyph + domain.DetailGlyphGap),
		Gap:     gap,
		Max:     domain.DashboardLogsJobColumnMax,
		TailMin: domain.DashboardLogsTailMin,
	})
	width := params.Width
	if colWidth > 0 {
		width = params.Width - colWidth - gap
	}
	return logsViewParams{
		Width:  max(width, 0),
		Height: max(params.Height-m.logsHeadRows()-domain.DashboardLogsChrome, 0),
	}
}

// logsViewBody is the logs view wherever it is hosted: the right-hand panel
// under its LOGS tab, and the Services tab at full width. One component, two
// hosts — the alternative was writing "show a tail" twice. The newest lines are
// the ones kept: this is a glance at what just happened, and runview is what
// scrolls.
func (m Model) logsViewBody(params logsViewParams) []string {
	if params.Width <= 0 {
		return nil
	}

	// The address keeps its row whether or not the job publishes one: a panel
	// whose body moves up and down as jobs are walked is a panel the eye has to
	// find again on every keystroke. No rule under it — the tab bar already
	// draws one two rows above, and a second so close reads as a box.
	head := append(m.logsAddressLines(params.Width), "")
	hint := styles.DashboardRowMeta.Render(truncate(domain.DashboardLogsHint, params.Width))
	tail := m.logsTailRect(params)
	budget := tail.Height
	colWidth := max(params.Width-tail.Width-lipgloss.Width(domain.DashboardLogsJobGap), 0)
	if tail.Width == params.Width {
		colWidth = 0
	}

	// Padded to its whole budget so the hint sits on the panel's last row: a
	// reminder that follows the tail lands somewhere new on every job.
	body := m.logsBodyLines(tail)
	for len(body) < budget {
		body = append(body, "")
	}

	return append(append(head, m.logsRows(logsRowsParams{
		Column: m.logsJobColumn(logsJobColumnParams{Width: colWidth, Rows: budget}),
		Tail:   body,
		Gap:    domain.DashboardLogsJobGap,
		Width:  params.Width,
	})...), "", hint)
}

type logsRowsParams struct {
	Column []string
	Tail   []string
	Gap    string
	Width  int
}

// logsRows lays the job column beside the tail it labels. A body too narrow for
// a column has none, and the tail then takes the whole width.
func (m Model) logsRows(params logsRowsParams) []string {
	if len(params.Column) == 0 {
		return params.Tail
	}
	// Rows are padded to the panel's width so the column keeps one edge down the
	// whole body rather than ending wherever the shortest log line does.
	rows := make([]string, 0, len(params.Tail))
	for index, line := range params.Tail {
		left := ""
		if index < len(params.Column) {
			left = params.Column[index]
		}
		rows = append(rows, pad(left+params.Gap+line, params.Width))
	}
	return rows
}

func (m Model) logsBody(layout domain.DashboardLayout) []string {
	return m.logsViewBody(logsViewParams{
		Width:  layout.Detail.Width - borderWidth - paddingWidth,
		Height: tabbedPanelBodyHeight(layout.Detail),
	})
}

type logsTailParams struct {
	Budget int
	Width  int
}

// logsBodyLines is the job's output as the panel shows it: the live preview
// when one is held — the same renderer and the same stream `run logs` uses —
// and the persisted tail otherwise, which is what a surface with no board falls
// back to.
func (m Model) logsBodyLines(tail logsViewParams) []string {
	if m.previewOn {
		return strings.Split(m.preview.View(), "\n")
	}
	return m.logsTailLines(logsTailParams{Budget: tail.Height, Width: tail.Width})
}

func (m Model) logsTailLines(params logsTailParams) []string {
	if empty := m.logsEmptyLines(params.Width); empty != nil {
		return empty
	}
	if m.logsErr != nil {
		return []string{styles.DashboardBlockers.Render(truncate(
			fmt.Sprintf(domain.DashboardUnavailableFmt, m.logsErr), params.Width))}
	}

	kept := m.logsLines[max(len(m.logsLines)-params.Budget, 0):]
	rendered := make([]string, 0, len(kept))
	for _, line := range kept {
		rendered = append(rendered, styles.DashboardValue.Render(truncate(line, params.Width)))
	}
	return rendered
}

// logsEmptyLines tells the four ways this view can have nothing to show apart,
// because the answer differs every time: a project with no run module needs
// `wtm run init`, a worktree that has started nothing needs a `run up`, a job
// that never ran here needs starting, and a job that ran and wrote nothing is
// simply quiet. Returns nil when there is a tail to draw.
//
// "Never ran" used to be inferred from the job not being up, which said it of a
// job that had run, been stopped, and written nothing. The log on disk is what
// actually answers the question, and it is now read rather than guessed at.
func (m Model) logsEmptyLines(width int) []string {
	if len(m.runConfig.Jobs) == 0 {
		return m.logsNotice(width, domain.DashboardLogsNoModule, domain.DashboardLogsNoModuleHint)
	}
	if len(m.logsJobs()) == 0 {
		return m.logsNotice(width, domain.DashboardLogsNothingRan, domain.DashboardLogsNothingRanHint)
	}
	if len(m.logsLines) > 0 || m.logsErr != nil {
		return nil
	}
	if m.jobIsUp(m.logsJob) {
		return m.logsNotice(width, domain.DashboardLogsQuiet, domain.DashboardLogsQuietHint)
	}
	if m.logged[m.logsBranch][m.logsJob] {
		return m.logsNotice(width, domain.DashboardLogsSilent, domain.DashboardLogsSilentHint)
	}
	return m.logsNotice(width, domain.DashboardLogsNeverRan, domain.DashboardLogsNeverRanHint)
}

func (m Model) logsNotice(width int, title, hint string) []string {
	return []string{
		styles.DashboardEmpty.Render(truncate(title, width)),
		"",
		styles.DashboardRowMeta.Render(truncate(hint, width)),
	}
}

func logsJobIndex(jobs []domain.JobConfig, name string) (index int, known bool) {
	for position, job := range jobs {
		if job.Name == name {
			return position, true
		}
	}
	return 0, false
}
