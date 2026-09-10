package runview

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
)

// View draws one frame as a fixed number of rows: the terminal holds nothing
// else, so a row too many shifts everything under it and a row too wide wraps.
func (m Model) View() string {
	layout := m.layout()
	if m.height <= 0 || m.width <= 0 {
		return ""
	}
	if m.preview {
		return strings.Join(fit(m.renderPreview(layout), m.height), "\n")
	}

	lines := make([]string, 0, m.height)
	lines = append(lines, blankRows(layout.GapRows)...)
	lines = append(lines, m.renderNotice(layout)...)
	lines = append(lines, m.renderBody(layout)...)
	lines = append(lines, blankRows(layout.GapRows)...)
	if layout.Help.Height > 0 {
		lines = append(lines, m.renderHelp(layout))
	}
	return strings.Join(indent(fit(lines, m.height), layout.MarginCols), "\n")
}

func blankRows(n int) []string {
	if n <= 0 {
		return nil
	}
	return make([]string, n)
}

// indent holds every row off the terminal's left edge. It is applied once, to
// the finished frame, rather than by each panel: a margin every renderer has to
// remember is a margin one of them forgets.
func indent(lines []string, margin int) []string {
	if margin <= 0 {
		return lines
	}
	pad := strings.Repeat(" ", margin)
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = pad + line
	}
	return out
}

// renderPreview is the hosted frame: the selected job's pane, filling the rect
// its host gave it. No header, no help, no list — the host draws its own, and
// the pane is the whole reason the preview exists.
func (m Model) renderPreview(layout domain.RunViewLayout) []string {
	if layout.Pane.Height < domain.RunViewMinPanelRows || layout.Pane.Width < domain.RunViewMinPanelCols {
		return make([]string, max(layout.Pane.Height, 0))
	}
	return fit(strings.Split(m.renderPanePanel(layout), "\n"), layout.Pane.Height)
}

// renderNotice draws the abort report as a band under the header. It takes rows
// from the body, never from what the panes hold: the output of the job that
// failed is the first thing the reader will want under it.
func (m Model) renderNotice(layout domain.RunViewLayout) []string {
	report := m.report()
	if layout.Notice.Height <= 0 || len(report) == 0 {
		return nil
	}

	shown := rules.ClipReport(report, layout.Notice.Height)
	lines := make([]string, 0, len(shown))
	for index, line := range shown {
		lines = append(lines, truncate(m.styleReportLine(index, len(shown), line), layout.Notice.Width))
	}
	return fit(lines, layout.Notice.Height)
}

func (m Model) styleReportLine(index, count int, line string) string {
	if index == 0 {
		return styles.DashboardDanger.Render(line)
	}
	if index == count-1 {
		return styles.Muted.Render(line)
	}
	return styles.Warning.Render(line)
}

// renderBody fills the rows between the header and the footer, and gives them up
// blank when there is not enough of them for a bordered panel to say anything.
func (m Model) renderBody(layout domain.RunViewLayout) []string {
	if layout.Pane.Height < domain.RunViewMinPanelRows || layout.Pane.Width < domain.RunViewMinPanelCols {
		return make([]string, max(layout.Pane.Height, 0))
	}

	body := m.renderPanePanel(layout)
	if layout.SidebarVisible {
		// Plain spaces rather than a styled block: a gutter carries no colour, and
		// a lipgloss.Style may only be instantiated in styles/.
		gutter := strings.Repeat(" ", max(layout.GutterCols, 0))
		body = lipgloss.JoinHorizontal(lipgloss.Top, m.renderSidebar(layout), gutter, body)
	}
	return fit(strings.Split(body, "\n"), layout.Pane.Height)
}

func (m Model) runningCount() int {
	running := 0
	for _, view := range m.jobs {
		if rules.IsJobUp(view.Status) {
			running++
		}
	}
	return running
}

func (m Model) renderSidebar(layout domain.RunViewLayout) string {
	width := layout.Sidebar.Width - domain.RunViewBorderWidth
	textWidth := width - 2
	lines := append(
		[]string{styles.Muted.Render(domain.RunViewJobsTitle), ""},
		m.renderJobRows(jobRowsParams{Width: textWidth, Rows: layout.SidebarRows})...,
	)

	return styles.RunViewSidebar.
		Width(width).
		Height(layout.Sidebar.Height - domain.RunViewBorderWidth).
		Render(strings.Join(lines, "\n"))
}

type jobRowsParams struct {
	Width int
	Rows  int
}

func (m Model) renderJobRows(params jobRowsParams) []string {
	all := m.rows()
	if len(all) == 0 {
		return []string{styles.Muted.Render(truncate(m.emptyMessage(), params.Width))}
	}

	now := time.Now()
	rendered := make([]string, 0, params.Rows)
	// The pinned heading costs the row it occupies: a group whose name has
	// scrolled away is worth more than the job that would have taken its place.
	if sticky := stickyHeader(all, m.offset); sticky != "" {
		rendered = append(rendered, styles.Muted.Render(truncate(sticky, params.Width)))
	}
	for _, row := range all[min(m.offset, len(all)):] {
		if len(rendered) == params.Rows {
			break
		}
		if row.Spacer {
			rendered = append(rendered, "")
			continue
		}
		if row.Header != "" {
			rendered = append(rendered, styles.Muted.Render(truncate(row.Header, params.Width)))
			continue
		}
		rendered = append(rendered, m.renderJobRow(jobRowParams{View: row.View, Width: params.Width, Now: now}))
	}
	return rendered
}

type jobRowParams struct {
	View  runlogs.JobView
	Width int
	Now   time.Time
}

func (m Model) renderJobRow(params jobRowParams) string {
	cursor, name := " ", m.indent()+params.View.Name
	if viewKey(params.View) == m.selected {
		cursor, name = styles.RunViewJobSelected.Render(domain.RunViewCursorMark), styles.RunViewJobSelected.Render(name)
	}
	uptime := rules.JobUptime(rules.JobUptimeParams{
		Job: domain.JobInfo{Status: params.View.Status, StartedAt: params.View.StartedAt},
		Now: params.Now,
	})

	return spread(spreadParams{
		Left:  cursor + m.jobMark(params.View) + " " + name,
		Right: styles.Muted.Render(uptime),
		Width: params.Width,
	})
}

func (m Model) jobMark(view runlogs.JobView) string {
	step, tracked := m.sequence.states[viewKey(view)]
	return renderMark(rules.JobMark(rules.JobMarkParams{Status: view.Status, Step: step, Tracked: tracked}))
}

func renderMark(mark domain.JobMark) string {
	switch mark {
	case domain.JobMarkStarting:
		return styles.Warning.Render(domain.RunViewMarkStarting)
	case domain.JobMarkRunning:
		return styles.Success.Render(domain.RunViewMarkRunning)
	case domain.JobMarkDetached:
		return styles.Success.Render(domain.RunViewMarkDetached)
	case domain.JobMarkShared:
		return styles.Success.Render(domain.RunViewMarkShared)
	case domain.JobMarkDone:
		return styles.Success.Render(domain.RunViewMarkDone)
	case domain.JobMarkCrashed:
		return styles.DangerText.Render(domain.RunViewMarkCrashed)
	default:
		return styles.Muted.Render(domain.RunViewMarkStopped)
	}
}

func (m Model) renderPanePanel(layout domain.RunViewLayout) string {
	view, found := m.selectedView()
	// The title is indented like the sidebar's, the body is not: every column
	// inside this border is one the emulator was sized for, and shifting the
	// output would misalign whatever the job draws.
	title := m.renderPaneTitle(paneTitleParams{
		View:  view,
		Found: found,
		Width: max(layout.PaneCols-2*domain.RunViewTitleIndent, 0),
	})
	if title != "" {
		title = strings.Repeat(" ", domain.RunViewTitleIndent) + title
	}
	// A blank row under the title, as the sidebar has under its own: the first
	// line a job writes is not a continuation of its name.
	lines := append([]string{title, ""},
		m.renderPaneBody(paneBodyParams{View: view, Found: found, Layout: layout})...)

	return m.paneStyle().
		Width(layout.Pane.Width - domain.RunViewBorderWidth).
		Height(layout.Pane.Height - domain.RunViewBorderWidth).
		Render(strings.Join(clip(lines, layout.Pane.Height-domain.RunViewBorderWidth), "\n"))
}

func (m Model) paneStyle() lipgloss.Style {
	if m.focused {
		return styles.RunViewPaneFocused
	}
	return styles.RunViewPane
}

type paneTitleParams struct {
	View  runlogs.JobView
	Found bool
	Width int
}

func (m Model) renderPaneTitle(params paneTitleParams) string {
	if !params.Found {
		return ""
	}
	status := m.statusWithAddress(params.View)
	left := styles.Bold.Render(m.qualify(params.View.Name, params.View.Worktree)) + styles.Muted.Render(domain.RunViewSeparator+status)
	return spread(spreadParams{Left: left, Right: styles.Muted.Render(m.paneOrigin(params.View)), Width: params.Width})
}

// statusWithAddress states where the job answers next to its status, preferring
// what this run observed to what the config predicts. Only `run up` has a
// sequence; `run logs` opens the same view with nothing started, and it used to
// show no address at all — the one difference between the two views.
func (m Model) statusWithAddress(view runlogs.JobView) string {
	key := viewKey(view)
	label := string(view.Status)

	// A url already carries the port it answers on, so the two are the same fact
	// twice — the rule JobAddressText states, applied here too. The observed url
	// still wins over the predicted one: only the run knows what the proxy really
	// served.
	if url := m.sequence.urls[key]; url != "" {
		return label + domain.RunViewSeparator + url
	}
	if ports := m.sequence.ports[key]; len(ports) > 0 {
		return rules.LabelWithPorts(rules.LabelWithPortsParams{Label: label, Ports: ports})
	}

	if address := rules.JobAddressText(view.Address); address != "" {
		return label + domain.RunViewSeparator + address
	}
	return label
}

// paneOrigin says what the pane is showing: the job as it prints, the log file
// it left behind, or a point in a history the reader has scrolled back into.
func (m Model) paneOrigin(view runlogs.JobView) string {
	if m.focused {
		return domain.RunViewFocusLabel
	}
	entry, held := m.panes.entry(viewKey(view))
	if !held {
		return ""
	}
	if offset := entry.pane.ScrollOffset(); offset > 0 {
		return fmt.Sprintf(domain.RunViewPaneScrollFmt, offset)
	}
	if entry.source == sourceHistory {
		return domain.RunViewPaneHistoryLabel
	}
	return domain.RunViewPaneLiveLabel
}

type paneBodyParams struct {
	View   runlogs.JobView
	Found  bool
	Layout domain.RunViewLayout
}

func (m Model) renderPaneBody(params paneBodyParams) []string {
	if !params.Found {
		return paneNote(m.emptyMessage(), params.Layout.PaneCols)
	}

	entry, held := m.panes.entry(viewKey(params.View))
	if !held {
		return paneNote(m.placeholder(params.View), params.Layout.PaneCols)
	}
	rendered := entry.pane.Render()
	if strings.TrimSpace(ansi.Strip(rendered)) == "" {
		return paneNote(m.placeholderFor(params.View, entry), params.Layout.PaneCols)
	}
	return strings.Split(rendered, "\n")
}

// paneNote is what the pane shows in place of output — a job with nothing to
// say yet, a view with no job at all. It is the view speaking rather than a
// job, so it is indented like the title above it; the emulator's own lines
// never are.
func paneNote(text string, width int) []string {
	pad := strings.Repeat(" ", domain.RunViewTitleIndent)
	return []string{pad + styles.Muted.Render(truncate(text, max(width-domain.RunViewTitleIndent, 0)))}
}

func (m Model) placeholderFor(view runlogs.JobView, entry jobPane) string {
	if entry.source == sourceHistory {
		return domain.RunViewPaneNoHistory
	}
	return m.placeholder(view)
}

func (m Model) placeholder(view runlogs.JobView) string {
	if view.Attachable {
		return domain.RunViewPaneWaiting
	}
	return domain.RunViewPaneNoHistory
}

func (m Model) emptyMessage() string {
	if m.filter != "" {
		return fmt.Sprintf(domain.RunViewNoMatchFmt, m.filter)
	}
	return domain.RunViewEmptyMessage
}

func (m Model) renderHelp(layout domain.RunViewLayout) string {
	if layout.Help.Height <= 0 {
		return ""
	}
	if m.focused {
		return styles.HelpBar.Render(truncate(
			fmt.Sprintf(domain.RunViewFocusHintFmt, m.selected, domain.RunViewFocusExitKey), layout.Help.Width))
	}
	if m.filtering {
		return styles.HelpBar.Render(truncate(
			domain.RunViewFilterPrompt+m.filter+styles.FilterCursor.Render(" ")+domain.RunViewSeparator+domain.RunViewHelpFilter, layout.Help.Width))
	}
	help := domain.RunViewHelpBrowse
	if m.filter != "" {
		help = fmt.Sprintf(domain.RunViewFilterHintFmt, m.filter) + domain.RunViewSeparator + help
	}
	// The run's state rides the help row rather than a header of its own: it is
	// one short string, and a row spent on it is a row of output. The hint gives
	// way to it whole segments at a time — a hint cut mid-word reads as a bug in
	// the hint.
	status := m.renderStatus(layout.Help.Width)
	room := max(layout.Help.Width-ansi.StringWidth(status)-1, 0)
	return spread(spreadParams{
		Left: styles.HelpBar.Render(rules.ClipSegments(rules.ClipSegmentsParams{
			Text:  help,
			Sep:   domain.RunViewSeparator,
			Width: room,
		})),
		Right: status,
		Width: layout.Help.Width,
	})
}

// runningLabel counts what is up, and how many worktrees it is spread over when
// that is more than one — the arity is what tells two jobs of the same name
// apart, and the view no longer has a header to say it in.
func (m Model) runningLabel() string {
	running := fmt.Sprintf(domain.RunViewRunningFmt, m.runningCount(), len(m.jobs))
	if count := m.worktreeCount(); count > 1 {
		return fmt.Sprintf(domain.RunStreamWorktreesFmt, count) + domain.RunViewSeparator + running
	}
	return running
}

// renderStatus is what the view has to say about itself, from the least urgent
// to the most: how much of it is up, which step a run has reached, a refusal it
// answered with, the error that ended it.
func (m Model) renderStatus(width int) string {
	status := styles.Muted.Render(m.runningLabel())
	if m.sequence.active {
		status = styles.Primary.Render(fmt.Sprintf(domain.RunViewStepFmt,
			m.sequence.step, m.sequence.steps, m.sequence.job))
	}
	if m.notice != "" {
		status = styles.Warning.Render(truncate(m.notice, width))
	}
	if m.err != nil {
		status = styles.DangerText.Render(truncate(m.err.Error(), width))
	}
	return status
}

type spreadParams struct {
	Left  string
	Right string
	Width int
}

// spread puts Right flush against the far edge and keeps it whole, cutting into
// Left when the row cannot hold both: Right is the shorter and the more urgent
// of the two — a state, a marker, a refusal — and dropping it to fit a hint that
// says the same thing on every frame is the wrong half to lose. A row too narrow
// for Right alone keeps Left instead, since something has to be shown.
func spread(params spreadParams) string {
	rightWidth := ansi.StringWidth(params.Right)
	if rightWidth == 0 {
		return truncate(params.Left, params.Width)
	}
	if rightWidth >= params.Width {
		return truncate(params.Left, params.Width)
	}

	left := truncate(params.Left, max(params.Width-rightWidth-1, 0))
	gap := params.Width - ansi.StringWidth(left) - rightWidth
	return left + strings.Repeat(" ", gap) + params.Right
}

func truncate(text string, width int) string {
	if width <= 0 {
		return ""
	}
	return ansi.Truncate(text, width, "")
}

func clip(lines []string, height int) []string {
	if height <= 0 || len(lines) <= height {
		return lines
	}
	return lines[:height]
}

// fit makes a block exactly height rows: what does not fit is dropped, what is
// missing is blank.
func fit(lines []string, height int) []string {
	if height <= 0 {
		return nil
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return clip(lines, height)
}
