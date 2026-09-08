package dashboard

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
	"github.com/LucasPcq/wtm/internal/tui/worktreepicker"
)

func (m Model) renderDetail(layout domain.DashboardLayout) string {
	width := max(layout.Detail.Width-borderWidth-paddingWidth, 0)
	return m.renderPanel(panelParams{
		Rect: layout.Detail,
		Body: append(append(m.panelTabLines(width), ""), m.detailBody(layout)...),
		Zone: zoneDetail,
	})
}

// The vital strip is instantaneous: it reads WorktreeStatus, not the lazily
// loaded WorktreeDetail, so it holds the panel while the rest arrives. Nothing
// here decides what to show — rules/ did — only how to stack and color it.
func (m Model) detailBody(layout domain.DashboardLayout) []string {
	width := layout.Detail.Width - borderWidth - paddingWidth
	status, ok := m.selected()
	if !ok {
		return []string{styles.DashboardEmpty.Render(truncate(domain.DashboardEmptySelection, width))}
	}
	if width <= 0 {
		return nil
	}
	if m.logsOpen() {
		return m.logsBody(layout)
	}

	stale := m.detailIsStale()
	detail, hasDetail := m.details[status.Branch]

	lines := []string{
		m.detailTitleLine(stale, status, width),
		styleText(stale, styles.DashboardRule, strings.Repeat(domain.DashboardRuleGlyph, width)),
		"",
	}
	lines = append(lines, vitalStripLines(rules.VitalChips(rules.VitalChipsParams{
		Status:       status,
		LastCommitAt: lastCommitAt(detail),
		Now:          time.Now(),
	}), width, stale)...)

	if hasDetail {
		if blocker := blockersLine(stale, detail.Blockers, width); blocker != "" {
			lines = append(lines, "", blocker)
		}
	}

	pr := m.prFor(status.Branch)
	budget := max(tabbedPanelBodyHeight(layout.Detail)-len(lines), 0)
	sections := m.detailSections(detailSectionsInput{
		Status:        status,
		Detail:        detail,
		HasDetail:     hasDetail,
		Parent:        m.parents[status.Branch],
		PR:            pr,
		PRUnavailable: m.prUnavailableReason(),
		RunConfig:     m.runConfig,
		Jobs:          m.jobs,
		Addresses:     m.addresses[status.Branch],
		Expanded:      m.runExpanded,
		AddressNote:   m.addressNotes[status.Branch],
		Height:        budget,
	})
	return m.appendSections(lines, sections, width, stale, pr)
}

// The title row carries identity, never state: the working-tree state is a
// vital-strip chip, not a title-row pill.
func (m Model) detailTitleLine(stale bool, status domain.WorktreeStatus, width int) string {
	branch := styleText(stale, styles.DashboardBranch, truncate(status.Branch, width))
	marker := m.youAreHereMarker(stale, status.Branch)
	if marker == "" {
		return branch
	}
	return spread(branch, marker, width)
}

func (m Model) youAreHereMarker(stale bool, branch string) string {
	if m.activeBranch == "" || branch != m.activeBranch {
		return ""
	}
	return styleText(stale, styles.DashboardValue, domain.DetailYouAreHere)
}

func lastCommitAt(detail domain.WorktreeDetail) time.Time {
	if len(detail.Commits) == 0 {
		return time.Time{}
	}
	return detail.Commits[0].At
}

// Wraps chip by chip and never cuts one mid-way: a truncated "origin ↑2 ↓…"
// would read as a lie rather than as a truncation.
func vitalStripLines(chips []domain.Chip, width int, stale bool) []string {
	if width <= 0 || len(chips) == 0 {
		return nil
	}
	sep := styleText(stale, styles.DashboardChipSep, domain.DashboardMetaSeparator)
	sepWidth := lipgloss.Width(domain.DashboardMetaSeparator)

	var lines []string
	var current string
	currentWidth := 0
	for _, chip := range chips {
		chipWidth := lipgloss.Width(chip.Text)
		rendered := renderChip(chip, stale)

		next := currentWidth + sepWidth + chipWidth
		if current != "" && next > width {
			lines = append(lines, current)
			current, currentWidth = rendered, chipWidth
			continue
		}
		if current == "" {
			current, currentWidth = rendered, chipWidth
			continue
		}
		current, currentWidth = current+sep+rendered, next
	}
	return append(lines, current)
}

// State gates WHETHER a chip is colored, Kind only WHICH color once it is:
// rules.stateChip stays the single source of truth for the strip's one
// colored chip, never re-derived here from Kind alone.
func renderChip(chip domain.Chip, stale bool) string {
	style := styles.DashboardChip
	if chip.State {
		switch chip.Kind {
		case domain.ChipKindDirty, domain.ChipKindRebasing:
			style = styles.Warning
		default:
			style = styles.Success
		}
	}
	return styleText(stale, style, chip.Text)
}

// Truncated before it is styled, like every other line here: trimming runes off
// an already-styled string eats its trailing reset and bleeds the color into
// whatever renders next.
func blockersLine(stale bool, blockers []domain.CleanBlocker, width int) string {
	if len(blockers) == 0 {
		return ""
	}
	labels := make([]string, 0, len(blockers))
	for _, blocker := range blockers {
		labels = append(labels, blocker.Label)
	}
	text := fmt.Sprintf(domain.DetailBlockedFmt, strings.Join(labels, domain.DashboardMetaSeparator))
	return styleText(stale, styles.DashboardBlockers, truncate(text, width))
}

// Carries the one thing rules.DetailSections cannot know: whether Detail has
// loaded at all yet.
type detailSectionsInput struct {
	Status    domain.WorktreeStatus
	Detail    domain.WorktreeDetail
	HasDetail bool
	Parent    string
	PR        *domain.PRInfo
	// PRUnavailable names why PR data could not be read (gh missing or not
	// authenticated), empty when it is fine. Set alongside PR so a broken tool
	// never renders as "no PR" (§8 state 4).
	PRUnavailable string
	// RunConfig is what the project declares it can run, Jobs what the daemon
	// holds right now. They are read straight off the model rather than off the
	// lazily loaded Detail: the jobs follow the poll, the addresses follow the
	// worktree.
	RunConfig domain.RunConfig
	Jobs      []domain.JobInfo
	Addresses map[string]domain.JobAddress
	// Expanded keys the runners whose children are showing.
	Expanded map[string]bool
	// AddressNote is what has to be said about those addresses, empty when the
	// worktree's .env answers on what its jobs publish.
	AddressNote string
	Height      int
}

// Which sections exist, their order and their placeholder lines are rules/'s
// job end to end; this only fits the finished stack to the panel's height.
func (m Model) detailSections(input detailSectionsInput) []domain.DetailSection {
	sections := rules.DetailSections(rules.DetailSectionsParams{
		Status:        input.Status,
		Detail:        input.Detail,
		DetailLoaded:  input.HasDetail,
		PR:            input.PR,
		PRUnavailable: input.PRUnavailable,
		RunConfig:     input.RunConfig,
		Jobs:          input.Jobs,
		Addresses:     input.Addresses,
		Expanded:      input.Expanded,
		AddressNote:   input.AddressNote,
		Parent:        input.Parent,
		Height:        input.Height,
		Now:           time.Now(),
	})
	return rules.FitSections(rules.FitSectionsParams{Sections: sections, Height: input.Height})
}

// The per-section spacing here is what rules.DetailSectionChrome accounts for;
// the two must agree or the panel overflows. REVIEW's first line takes a mouse
// zone only when pr is set: the same line also renders a failure placeholder,
// and that one has nothing to open.
func (m Model) appendSections(lines []string, sections []domain.DetailSection, width int, stale bool, pr *domain.PRInfo) []string {
	for _, section := range sections {
		lines = append(lines, "", sectionTitleLine(stale, section, width), "")
		if section.Rows != nil {
			lines = append(lines, m.runRowLines(section, width, stale)...)
			continue
		}
		for index, line := range section.Lines {
			rendered := styleText(stale, styles.DashboardValue, truncate(line, width))
			if pr != nil && section.Key == domain.DetailSectionReview && index == 0 {
				rendered = m.marks().Mark(zoneDetailPR, rendered)
			}
			lines = append(lines, rendered)
		}
	}
	return lines
}

// Only an up row takes a zone: a stopped job answers nowhere, the same rule
// REVIEW's first line follows when there is no PR to open.
func (m Model) runRowLines(section domain.DetailSection, width int, stale bool) []string {
	lines := sectionRowLines(sectionRowLinesParams{
		Rows: section.Rows, Width: width, Stale: stale,
		MarkAddress: func(row domain.DetailRow, cell string) string {
			return m.marks().Mark(runURLZone(row.Key), cell)
		},
	})
	for index, row := range section.Rows {
		if index >= len(lines) {
			continue
		}
		// A foldable row takes the fold zone: it is the outer one, so the address
		// cell inside it keeps answering first.
		if row.Fold {
			lines[index] = m.marks().Mark(runFoldZone(row.Key), lines[index])
			continue
		}
		if !row.Up {
			continue
		}
		lines[index] = m.marks().Mark(runRowZone(row.Key), lines[index])
	}
	return lines
}

type sectionRowLinesParams struct {
	Rows  []domain.DetailRow
	Width int
	Stale bool
	// MarkAddress wraps the address cell in its own mouse zone, after it has
	// been truncated and styled — never before, or spread could cut through the
	// marker. Nil for a caller that marks nothing.
	MarkAddress func(row domain.DetailRow, cell string) string
	// NameWidth sizes the name column from outside, for a surface stacking
	// several groups of rows that must read as one table. Zero sizes it on the
	// rows given, which is what a single section wants.
	NameWidth int
}

// sectionRowLines sizes the name column on its widest cell and lays the meta
// flush right, the same way a panel's title row does: a table only reads down
// its columns once every cell knows how wide its column ended up.
func sectionRowLines(params sectionRowLinesParams) []string {
	nameWidth := params.NameWidth
	for _, row := range params.Rows {
		nameWidth = max(nameWidth, len([]rune(cellText(row, domain.DetailCellName))))
	}

	lines := make([]string, 0, len(params.Rows))
	for _, row := range params.Rows {
		if note := cellText(row, domain.DetailCellNote); note != "" {
			indent := domain.DetailListIndent
			lines = append(lines, styleText(params.Stale, styles.DashboardRowMeta,
				indent+truncate(note, max(params.Width-len(indent), 0))))
			continue
		}
		if warn := cellText(row, domain.DetailCellWarn); warn != "" {
			lines = append(lines, warnLine(warnLineParams{Text: warn, Width: params.Width, Stale: params.Stale}))
			continue
		}
		if hasCell(row, domain.DetailCellGap) {
			lines = append(lines, "")
			continue
		}
		meta := rowMetaCell(row, params.Stale)
		lines = append(lines, spread(rowLeft(rowLeftParams{
			Row:         row,
			NameWidth:   nameWidth,
			Stale:       params.Stale,
			Budget:      max(params.Width-lipgloss.Width(meta)-1, 0),
			MarkAddress: params.MarkAddress,
		}), meta, params.Width))
	}
	return lines
}

type warnLineParams struct {
	Text  string
	Width int
	Stale bool
}

// warnLine is the one line a panel gives to something it has to be sure is
// read: the glyph and the warning colour set it apart from the muted asides it
// sits under, and the caller puts the blank line above it.
func hasCell(row domain.DetailRow, kind domain.DetailCellKind) bool {
	for _, cell := range row.Cells {
		if cell.Kind == kind {
			return true
		}
	}
	return false
}

func warnLine(params warnLineParams) string {
	indent := domain.DetailListIndent + domain.AddressingDriftGlyph
	return styleText(params.Stale, styles.Warning,
		indent+truncate(params.Text, max(params.Width-lipgloss.Width(indent), 0)))
}

type rowLeftParams struct {
	Row         domain.DetailRow
	NameWidth   int
	Stale       bool
	MarkAddress func(row domain.DetailRow, cell string) string
	// Budget is what the left side may occupy. The address is cut to it here,
	// with an ellipsis: spread would clip it silently, and a clipped url reads
	// as a whole one — on a row whose click opens the real address.
	Budget int
}

func rowLeft(params rowLeftParams) string {
	glyphStyle := styles.DashboardRowMeta
	if params.Row.Up {
		glyphStyle = styles.Success
	}
	// A child hangs under its runner rather than beside it: it has no state of
	// its own to mark, so the glyph column becomes its indent.
	head := domain.DetailListIndent + strings.Repeat(domain.DetailHeldIndent, params.Row.Depth) +
		styleText(params.Stale, glyphStyle, cellText(params.Row, domain.DetailCellGlyph)) +
		domain.DetailGlyphGap +
		styleText(params.Stale, styles.DashboardRowMeta,
			pad(cellText(params.Row, domain.DetailCellName), params.NameWidth))

	address := truncate(cellText(params.Row, domain.DetailCellAddress),
		max(params.Budget-lipgloss.Width(head)-lipgloss.Width(domain.DetailColumnGap), 0))
	if address == "" {
		return head
	}
	style := styles.DashboardRowMeta
	if params.Row.URL != "" {
		style = styles.DashboardURL
	}
	cell := styleText(params.Stale, style, address)
	if params.MarkAddress != nil && params.Row.URL != "" {
		cell = params.MarkAddress(params.Row, cell)
	}
	return head + domain.DetailColumnGap + cell
}

func rowMetaCell(row domain.DetailRow, stale bool) string {
	meta := cellText(row, domain.DetailCellMeta)
	if fold := cellText(row, domain.DetailCellFold); fold != "" {
		meta = strings.TrimSpace(fold + domain.DetailGlyphGap + meta)
	}
	if meta == "" {
		return ""
	}
	return styleText(stale, styles.DashboardRowMeta, meta)
}

func cellText(row domain.DetailRow, kind domain.DetailCellKind) string {
	for _, cell := range row.Cells {
		if cell.Kind == kind {
			return cell.Text
		}
	}
	return ""
}

// Off the UI goroutine: PROpener shells out to gh, and calling it inside Update
// would freeze the program. Failures take openPRMsg to the output panel.
func (m Model) openPR() (Model, tea.Cmd) {
	pr := m.prFor(m.selectedBranch())
	if pr == nil || m.params.PROpener == nil {
		return m, nil
	}
	opener, number := m.params.PROpener, pr.Number
	return m, func() tea.Msg {
		return openPRMsg{err: opener(number)}
	}
}

// sectionTitleLine mirrors panelParams.TitleRight (render.go): TitleRight is
// dropped whole, never trimmed through, when the panel is too narrow for it.
func sectionTitleLine(stale bool, section domain.DetailSection, width int) string {
	if section.TitleRight == "" {
		return styleText(stale, styles.DashboardSectionTitle, truncate(section.Title, width))
	}
	rightWidth := lipgloss.Width(section.TitleRight)
	if rightWidth+1 >= width {
		return styleText(stale, styles.DashboardSectionTitle, truncate(section.Title, width))
	}
	left := styleText(stale, styles.DashboardSectionTitle, truncate(section.Title, width-rightWidth-1))
	right := styleText(stale, styles.DashboardValue, section.TitleRight)
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	return left + strings.Repeat(" ", max(gap, 0)) + right
}

// A stale body renders uniformly muted: old data still on screen, visibly not
// the freshest read.
func styleText(stale bool, style lipgloss.Style, text string) string {
	if stale {
		return styles.DashboardStale.Render(text)
	}
	return style.Render(text)
}

// Empty until prsMsg lands with a real connection state, so no "unavailable"
// claim is made while the fetch is still in flight.
func (m Model) prUnavailableReason() string {
	return worktreepicker.GHBanner(m.ghConn).Title
}

// REVIEW reads the already-loaded PR list, not the lazily loaded Detail.
func (m Model) prFor(branch string) *domain.PRInfo {
	for index := range m.prs {
		if m.prs[index].Branch == branch {
			return &m.prs[index]
		}
	}
	return nil
}
