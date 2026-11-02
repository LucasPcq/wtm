package runview

import (
	"fmt"
	"slices"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/ui/output"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// View draws the frame, exactly as many rows as the terminal has: the band of
// notices, the list beside the pane, the help row.
func (m *model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	l := m.layout()

	lines := make([]string, l.gap, m.height)
	lines = append(lines, m.drawBand(l)...)
	lines = append(lines, m.drawBody(l)...)
	lines = append(lines, make([]string, l.gap)...)
	if l.help.height > 0 {
		lines = append(lines, m.drawHelp(l.help.width))
	}

	pad := strings.Repeat(" ", l.margin)
	lines = fit(lines, m.height)
	for i, line := range lines {
		lines[i] = pad + line
	}
	return strings.Join(lines, "\n")
}

// drawBand draws the notices, the last row saying how to dismiss them, kept
// when the frame is too short for them all.
func (m *model) drawBand(l layout) []string {
	band := m.band()
	if l.notice.height == 0 || len(band) == 0 {
		return nil
	}
	switch {
	case l.notice.height == 1:
		band = band[:1]
	case len(band) > l.notice.height:
		band = append(band[:l.notice.height-1:l.notice.height-1], band[len(band)-1])
	}

	lines := make([]string, len(band))
	for i, text := range band {
		if i == len(band)-1 {
			lines[i] = m.st.Muted.Render(truncate(text, l.notice.width))
			continue
		}
		lines[i] = truncate(m.st.Warning.Render(glyphNotice)+" "+text, l.notice.width)
	}
	return lines
}

func (m *model) drawBody(l layout) []string {
	if l.pane.height < minPanelRows || l.pane.width < minPanelCols {
		return make([]string, l.pane.height)
	}
	body := m.drawPane(l)
	if l.sidebar.width > 0 {
		body = lipgloss.JoinHorizontal(lipgloss.Top, m.drawList(l), strings.Repeat(" ", l.gutter), body)
	}
	return fit(strings.Split(body, "\n"), l.pane.height)
}

// ----------------------------------------------------------------------------
// The list

func (m *model) drawList(l layout) string {
	width := l.sidebar.width - borderWidth
	lines := append([]string{m.st.Muted.Render(jobsTitle), ""}, m.drawRows(width-2, l.listRows)...)

	return m.box(false).Padding(0, 1).Width(width).Height(l.sidebar.height - borderWidth).Render(strings.Join(lines, "\n"))
}

// drawRows draws the rows of the list from its offset, the heading of the
// group it opens in pinned on top once its own scrolled away.
func (m *model) drawRows(width, height int) []string {
	all := rows(m.visible())
	if len(all) == 0 {
		return []string{m.st.Muted.Render(truncate(m.empty(), width))}
	}

	var drawn []string
	if pinned, found := pinnedHeading(all, m.offset); found {
		drawn = append(drawn, m.heading(pinned, width))
	}
	for _, r := range all[min(m.offset, len(all)):] {
		if len(drawn) == height {
			break
		}
		switch {
		case r.spacer:
			drawn = append(drawn, "")
		case r.entry == nil:
			drawn = append(drawn, m.heading(r, width))
		default:
			drawn = append(drawn, m.jobRow(*r.entry, width))
		}
	}
	return drawn
}

func pinnedHeading(all []row, offset int) (row, bool) {
	if offset <= 0 || offset >= len(all) || all[offset].entry == nil {
		return row{}, false
	}
	for i := offset - 1; i >= 0; i-- {
		if all[i].entry == nil && !all[i].spacer {
			return all[i], true
		}
	}
	return row{}, false
}

func (m *model) heading(r row, width int) string {
	style := m.st.NewStyle().Bold(true)
	if r.shared {
		style = style.Foreground(output.ColorSuccess)
	}
	return style.Render(truncate(r.heading, width))
}

func (m *model) jobRow(e entry, width int) string {
	cursor, name := " ", " "+e.name
	if e.key == m.selected {
		selected := m.st.NewStyle().Foreground(output.ColorPrimary).Bold(true)
		cursor, name = selected.Render(glyphCursor), selected.Render(name)
	}
	return spread(cursor+m.mark(markOf(e))+name, m.st.Muted.Render(uptime(e.job, m.now())), width)
}

func (m *model) mark(of mark) string {
	switch of {
	case markStarting:
		return m.st.Warning.Render(glyphStarting)
	case markUp:
		return m.st.Success.Render(glyphUp)
	case markDone:
		return m.st.Success.Render(glyphDone)
	case markFailed:
		return m.st.Danger.Render(glyphFailed)
	case markStopped:
	}
	return m.st.Muted.Render(glyphStopped)
}

func (m *model) empty() string {
	if m.filter != "" {
		return fmt.Sprintf(noMatchFmt, m.filter)
	}
	return noJobs
}

// ----------------------------------------------------------------------------
// The pane

func (m *model) drawPane(l layout) string {
	e, found := m.selectedEntry()
	title, body := "", []string{m.note(m.empty(), l.paneCols)}
	switch {
	case m.reaching:
		title, body = m.st.NewStyle().Bold(true).Render(reachTitle), m.reach(l.paneCols)
	case found:
		title, body = m.title(e, l.paneCols-2*titleIndent), m.output(e, l.paneCols)
	}
	if title != "" {
		title = strings.Repeat(" ", titleIndent) + title
	}
	lines := append([]string{title, ""}, body...)
	if inner := l.pane.height - borderWidth; len(lines) > inner {
		lines = lines[:max(inner, 0)]
	}

	return m.box(m.focused).Width(l.pane.width - borderWidth).Height(l.pane.height - borderWidth).Render(strings.Join(lines, "\n"))
}

func (m *model) box(focused bool) lipgloss.Style {
	border := output.ColorMuted
	if focused {
		border = output.ColorPrimary
	}
	return m.st.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(border)
}

// title names the job, by its worktree too unless it is a shared service, its
// state and its address, and on the right what the pane shows of it.
func (m *model) title(e entry, width int) string {
	name := e.name
	if !e.shared {
		name += separator + e.branch
	}
	state := m.state(e)
	if reached := m.reached(e); reached != "" && e.job.Up {
		state += separator + reached
	}
	return spread(m.st.NewStyle().Bold(true).Render(name)+m.st.Muted.Render(separator+state), m.st.Muted.Render(m.origin(e)), width)
}

// origin says what the pane shows: the job as it prints, its log, or a point
// the reader scrolled back to.
func (m *model) origin(e entry) string {
	switch {
	case m.focused:
		return labelFocus
	case m.pane == nil || m.bound.key != e.key || m.pending || m.failed != nil:
		return ""
	case m.pane.scrolled() > 0:
		return fmt.Sprintf(scrolledFmt, m.pane.scrolled())
	case m.bound.live && !m.ended:
		return labelLive
	}
	return labelHistory
}

func (m *model) output(e entry, width int) []string {
	if !e.listed {
		return []string{m.note(paneNotStarted, width)}
	}
	if m.pane == nil || m.bound.key != e.key {
		return []string{m.note(paneNoHistory, width)}
	}
	if m.failed != nil {
		return []string{m.note(fmt.Sprintf(paneFailedFmt, m.failed), width)}
	}
	drawn := m.pane.render()
	if strings.TrimSpace(ansi.Strip(drawn)) == "" {
		if m.bound.live {
			return []string{m.note(paneWaiting, width)}
		}
		return []string{m.note(paneNoHistory, width)}
	}
	return strings.Split(drawn, "\n")
}

// note is the view speaking in the pane, in place of a job's output.
func (m *model) note(text string, width int) string {
	return strings.Repeat(" ", titleIndent) + m.st.Muted.Render(truncate(text, width-titleIndent))
}

// reach draws where each job up answers: the shared services, then each
// worktree's jobs, a job with no address left out.
func (m *model) reach(width int) []string {
	var lines []string
	group := ""
	for _, e := range entries(m.params, m.jobs, "") {
		if !e.job.Up || e.job.URL == "" && len(e.job.Ports) == 0 {
			continue
		}
		heading := e.branch
		if e.shared {
			heading = sharedHeading
		}
		if heading != group {
			if group != "" {
				lines = append(lines, "")
			}
			group = heading
			lines = append(lines, m.heading(row{heading: heading, shared: e.shared}, width))
		}
		line := output.Indent + e.name + "  " + m.address(e)
		if namespaces := m.namespaces(e); e.shared && len(namespaces) > 0 {
			line += separator + strings.Join(namespaces, ", ")
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return []string{m.note(reachNothing, width)}
	}
	pad := strings.Repeat(" ", titleIndent)
	for i, line := range lines {
		lines[i] = pad + truncate(line, width-titleIndent)
	}
	return lines
}

// namespaces are those the worktrees of the view hold in the shared service of e.
func (m *model) namespaces(e entry) []string {
	var held []string
	for _, wt := range m.params.Worktrees {
		if name := wt.Namespaces[e.name]; name != "" {
			held = append(held, name)
		}
	}
	return held
}

// address is a job's URL, else the ports it binds.
func (m *model) address(e entry) string {
	if e.job.URL != "" {
		return link(e.job.URL)
	}
	var ports []string
	for _, name := range sortedKeys(e.job.Ports) {
		ports = append(ports, fmt.Sprintf("%s :%d", name, e.job.Ports[name]))
	}
	return strings.Join(ports, separator)
}

// reached is where a job answers, in its title: its URL, else its ports.
func (m *model) reached(e entry) string {
	if e.job.URL != "" {
		return link(e.job.URL)
	}
	var ports []string
	for _, name := range sortedKeys(e.job.Ports) {
		ports = append(ports, fmt.Sprintf(":%d", e.job.Ports[name]))
	}
	return strings.Join(ports, " ")
}

// link makes url a hyperlink the terminal opens with its own gesture.
func link(url string) string {
	return ansi.SetHyperlink(url) + url + ansi.ResetHyperlink()
}

// ----------------------------------------------------------------------------
// The help row

// drawHelp is the keys on the left, cut at a whole segment, and on the right
// what the view has to say: how many jobs run, else what it answered a key
// with, else the error it met.
func (m *model) drawHelp(width int) string {
	if m.focused {
		e, _ := m.selectedEntry()
		return m.st.Muted.Render(truncate(fmt.Sprintf(focusHintFmt, e.name, e.branch), width))
	}
	if m.filtering {
		return m.st.Muted.Render(truncate(filterPrompt+m.filter+m.st.NewStyle().Reverse(true).Render(" ")+separator+helpFilter, width))
	}

	help := helpBrowse
	if m.filter != "" {
		help = fmt.Sprintf(filterHintFmt, m.filter) + separator + help
	}
	status := m.status(width)
	return spread(m.st.Muted.Render(segments(help, max(width-ansi.StringWidth(status)-1, 0))), status, width)
}

func (m *model) status(width int) string {
	switch {
	case m.err != nil:
		return m.st.Danger.Render(truncate(m.err.Error(), width))
	case m.failed != nil:
		return m.st.Danger.Render(truncate(m.failed.Error(), width))
	case m.said != "":
		return m.st.Warning.Render(truncate(m.said, width))
	}
	list := entries(m.params, m.jobs, "")
	running := 0
	for _, e := range list {
		if e.job.Up {
			running++
		}
	}
	text := fmt.Sprintf(runningFmt, running, len(list))
	if len(m.params.Worktrees) > 1 {
		text = fmt.Sprintf(worktreesFmt, len(m.params.Worktrees)) + separator + text
	}
	return m.st.Muted.Render(text)
}

// segments cuts text at the last whole segment that fits in width.
func segments(text string, width int) string {
	if ansi.StringWidth(text) <= width {
		return text
	}
	kept := ""
	for _, segment := range strings.Split(text, separator) {
		next := segment
		if kept != "" {
			next = kept + separator + segment
		}
		if ansi.StringWidth(next) > width {
			break
		}
		kept = next
	}
	return kept
}

// ----------------------------------------------------------------------------
// Helpers

// spread puts right against the far edge, whole, and cuts into left to make
// room: right is the shorter and the more urgent.
func spread(left, right string, width int) string {
	rightWidth := ansi.StringWidth(right)
	if rightWidth == 0 || rightWidth >= width {
		return truncate(left, width)
	}
	left = truncate(left, width-rightWidth-1)
	return left + strings.Repeat(" ", width-ansi.StringWidth(left)-rightWidth) + right
}

func truncate(text string, width int) string {
	if width <= 0 {
		return ""
	}
	return ansi.Truncate(text, width, "")
}

// fit makes lines exactly height rows.
func fit(lines []string, height int) []string {
	for len(lines) < height {
		lines = append(lines, "")
	}
	return lines[:max(height, 0)]
}

func sortedKeys(ports map[string]int) []string {
	names := make([]string, 0, len(ports))
	for name := range ports {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}
