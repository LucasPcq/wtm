package dashboard

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
)

func (m Model) renderServices(layout domain.DashboardLayout) string {
	return m.renderPanel(panelParams{
		Rect:  layout.List,
		Title: domain.DashboardServicesTitle,
		Body:  m.servicesBody(layout),
		Zone:  zoneServices,
	})
}

// servicesBlocks is what the daemon holds up, read off the poll: jobs and
// addresses, never a lazily loaded detail.
func (m Model) servicesBlocks() []rules.RunWorktreeBlock { return m.board }

// nearestServiceJob is the first job row from index in the given direction,
// then in the other: a header and a gap are drawn, never selected.
type nearestJobParams struct {
	Index     int
	Direction int
}

func (m Model) nearestServiceJob(params nearestJobParams) int {
	index, direction := params.Index, params.Direction
	if len(m.services) == 0 {
		return 0
	}
	index = rules.ClampIndex(index, len(m.services))
	for _, step := range []int{direction, -direction} {
		for cursor := index; cursor >= 0 && cursor < len(m.services); cursor += step {
			if m.services[cursor].Kind == domain.ServicesRowJob {
				return cursor
			}
		}
	}
	return index
}

func (m Model) selectedService() (domain.ServicesRow, bool) {
	if m.servicesCursor < 0 || m.servicesCursor >= len(m.services) {
		return domain.ServicesRow{}, false
	}
	row := m.services[m.servicesCursor]
	return row, row.Kind == domain.ServicesRowJob
}

// stepServices walks job rows, never landing on a header: the cursor's subject
// is a job, and a header is a caption over several of them.
func (m Model) stepServices(delta int) Model {
	if delta == 0 || len(m.services) == 0 {
		return m
	}
	direction := 1
	if delta < 0 {
		direction = -1
	}
	cursor := m.servicesCursor
	for range max(delta, -delta) {
		next := m.nearestServiceJob(nearestJobParams{Index: cursor + direction, Direction: direction})
		if next == cursor {
			break
		}
		cursor = next
	}
	m.servicesCursor = cursor
	return m
}

// servicesVisible is the window the tab draws, cursor included.
func (m Model) servicesVisible(layout domain.DashboardLayout) []domain.ServicesRow {
	rows := layout.ServicesRows
	if rows <= 0 || len(m.services) == 0 {
		return nil
	}
	end := min(m.servicesOffset+rows, len(m.services))
	if m.servicesOffset >= end {
		return nil
	}
	return m.services[m.servicesOffset:end]
}

// servicesBody stacks one block per worktree: its name and count on a header
// row, then the same typed rows the detail panel's RUN section draws, so the
// two surfaces cannot read differently.
func (m Model) servicesBody(layout domain.DashboardLayout) []string {
	width := layout.List.Width - borderWidth - paddingWidth
	if width <= 0 {
		return nil
	}

	if len(m.services) == 0 {
		return m.servicesEmptyLines(width)
	}

	// The name column is sized across every block, not per block: the addresses
	// are what this tab is read down, and a column restarting at each worktree
	// would break that read.
	nameWidth := 0
	for _, row := range m.services {
		if row.Kind == domain.ServicesRowJob || row.Kind == domain.ServicesRowHeld {
			nameWidth = max(nameWidth, len([]rune(cellText(row.Job, domain.DetailCellName))))
		}
	}

	visible := m.servicesVisible(layout)
	lines := make([]string, 0, len(visible))
	for position, row := range visible {
		index := m.servicesOffset + position
		switch row.Kind {
		case domain.ServicesRowGap:
			lines = append(lines, "")
		case domain.ServicesRowNote:
			lines = append(lines, warnLine(warnLineParams{Text: row.Note, Width: width}))
		case domain.ServicesRowHeader:
			lines = append(lines, spread(
				styles.DashboardRowName.Render(row.Branch),
				styles.DashboardRowMeta.Render(fmt.Sprintf(domain.DashboardServicesUpFmt, row.Up)),
				width,
			))
		case domain.ServicesRowJob, domain.ServicesRowHeld:
			line := sectionRowLines(sectionRowLinesParams{
				Rows: []domain.DetailRow{row.Job}, Width: width, NameWidth: nameWidth,
				MarkAddress: func(_ domain.DetailRow, cell string) string {
					return m.marks().Mark(servicesURLZone(index), cell)
				},
			})[0]
			// A held address is drawn like a job and selected like none: the cursor
			// and the menu act on processes, and the runner above owns this one.
			if row.Kind == domain.ServicesRowHeld {
				lines = append(lines, line)
				continue
			}
			line = m.servicesJobLine(servicesJobLineParams{Index: index, Line: line})
			if row.Job.Fold {
				line = m.marks().Mark(servicesFoldZone(index), line)
			}
			lines = append(lines, m.marks().Mark(servicesRowZone(index), line))
		}
	}
	return lines
}

// watchServiceLogs opens the full run view on the job under the cursor. The tab
// used to turn its own body into a smaller logs view first, which meant two
// keystrokes and a second stop on the way to the same place: the overview is
// what this tab is for, and a job picked out of it is a job one wants to read.
func (m Model) watchServiceLogs() (Model, tea.Cmd) {
	row, ok := m.selectedService()
	if !ok {
		return m, nil
	}
	m.logsBranch, m.logsJob = row.Branch, row.Job.Key
	return m.watchLogs()
}

// servicesJobLine marks the row under the cursor the way a list row is marked:
// the accent bar in its gutter, so the two tabs read as one dashboard.
type servicesJobLineParams struct {
	Index int
	Line  string
}

func (m Model) servicesJobLine(params servicesJobLineParams) string {
	index, line := params.Index, params.Line
	if index != m.servicesCursor {
		return line
	}
	return styles.DashboardRowBar.Render(rowBar) + strings.TrimPrefix(line, " ")
}

// servicesEmptyLines names every route into this tab, the single job included:
// naming only the profile is what the first version did.
func (m Model) servicesEmptyLines(width int) []string {
	command := 0
	for _, row := range domain.DashboardServicesEmptyRows {
		command = max(command, len([]rune(row[0])))
	}

	lines := []string{styles.DashboardEmpty.Render(truncate(domain.DashboardServicesEmpty, width)), ""}
	for _, row := range domain.DashboardServicesEmptyRows {
		head := domain.DetailListIndent + pad(truncate(row[0], width), command)
		budget := max(width-lipgloss.Width(head)-lipgloss.Width(domain.DetailColumnGap), 0)
		lines = append(lines,
			styles.DashboardValue.Render(head)+
				domain.DetailColumnGap+
				styles.DashboardRowMeta.Render(truncate(row[1], budget)))
	}
	return lines
}

// clickServiceRow answers a click on a Services job row. Only the address cell
// acts: a click anywhere else moves the cursor and no further. Opening a log is
// a full change of view, and the rows are close enough together that clicking
// one by accident was the common case — enter does it deliberately.
func (m Model) clickServiceRow(msg tea.MouseMsg) (tea.Model, tea.Cmd, bool) {
	if m.tab != tabServices {
		return m, nil, false
	}
	for index, row := range m.services {
		if row.Kind != domain.ServicesRowJob && row.Kind != domain.ServicesRowHeld {
			continue
		}
		if m.inZone(servicesURLZone(index), msg) {
			model, cmd := m.openJobURL(row.Job.URL)
			return model, cmd, true
		}
		if row.Job.Fold && m.inZone(servicesFoldZone(index), msg) {
			// withBoard, not reflow: this tab's rows are built once and cached, so
			// the fold only shows once the board is rebuilt around it.
			return m.toggleRunFold(row.Job.Key).withBoard(), nil, true
		}
		if row.Kind == domain.ServicesRowHeld || !m.inZone(servicesRowZone(index), msg) {
			continue
		}
		m.servicesCursor = index
		return m.reflow(), nil, true
	}
	return m, nil, false
}

// selectServiceRow puts the cursor on the job row under the pointer, and says
// whether there was one. It is what the right button needs: the menu acts on the
// job it was opened over, and a menu acting on another row would be a trap.
func (m Model) selectServiceRow(msg tea.MouseMsg) (Model, bool) {
	for index, row := range m.services {
		if row.Kind != domain.ServicesRowJob || !m.inZone(servicesRowZone(index), msg) {
			continue
		}
		m.servicesCursor = index
		return m.reflow(), true
	}
	return m, false
}
