package components

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
)

// SelectItem represents a single entry in a SelectList.
type SelectItem struct {
	Label string
	Value string
	// Badges are left-column tags rendered as colored text, aligned to a fixed
	// column across rows so they line up regardless of label length.
	Badges []Badge
	// Status, when set, is a filled status pill right-aligned to the row width.
	Status    *Badge
	Disabled  bool
	Danger    bool
	Separator bool
}

// SelectListModel is a navigable list with full-row highlight and inline filtering.
type SelectListModel struct {
	items     []SelectItem
	filtered  []int
	cursor    int
	filter    string
	filtering bool
	width     int
	height    int
	offset    int
	title     string
	desc      string
	chosen    bool
	aborted   bool
}

// NewSelectList creates a SelectList with the given title, description, and items.
func NewSelectList(params NewSelectListParams) SelectListModel {
	m := SelectListModel{
		items: params.Items,
		title: params.Title,
		desc:  params.Description,
		width: 80,
	}
	m.refilter()
	m.snapToSelectable()
	m.startOn(params.Start)
	return m
}

// NewSelectListParams holds inputs for NewSelectList.
type NewSelectListParams struct {
	Title       string
	Description string
	Items       []SelectItem
	// Start is the value the cursor opens on. An empty or unknown one leaves it
	// on the first selectable item.
	Start string
}

// startOn places the cursor on the named value, so a list with a standing answer
// opens on it instead of making the reader find it.
func (m *SelectListModel) startOn(value string) {
	if value == "" {
		return
	}
	for i, idx := range m.filtered {
		if m.items[idx].Value == value && !m.items[idx].Separator {
			m.cursor = i
			return
		}
	}
}

// Chosen returns true after the user confirmed a selection.
func (m SelectListModel) Chosen() bool { return m.chosen }

// Aborted returns true after the user pressed Esc (outside filter mode).
func (m SelectListModel) Aborted() bool { return m.aborted }

// Value returns the selected item's value.
func (m SelectListModel) Value() string {
	if m.cursor >= 0 && m.cursor < len(m.filtered) {
		return m.items[m.filtered[m.cursor]].Value
	}
	return ""
}

// SetBadges replaces the badges of items whose Value is a key in byValue,
// preserving cursor, filter, and scroll position. Used to refresh rows when
// async data (e.g. PRs) arrives after the list is already on screen.
func (m *SelectListModel) SetBadges(byValue map[string][]Badge) {
	for i := range m.items {
		if badges, ok := byValue[m.items[i].Value]; ok {
			m.items[i].Badges = badges
		}
	}
}

// SetItems replaces the list items, resetting cursor, scroll, and filter while
// preserving the configured width/height. Used to populate a list whose items
// arrive asynchronously (e.g. PRs streamed into the checkout picker). It is to
// items what SetBadges is to badges.
func (m *SelectListModel) SetItems(items []SelectItem) {
	m.items = items
	m.cursor = 0
	m.offset = 0
	m.filter = ""
	m.filtering = false
	m.refilter()
	m.snapToSelectable()
}

type SetSizeParams struct {
	Width  int
	Height int
}

// SetSize fits the list to the region its host gives it. The wizard sizes its
// steps directly; a host outside this package (the dashboard modal) goes through
// here.
func (m *SelectListModel) SetSize(params SetSizeParams) {
	m.width, m.height = params.Width, params.Height
	m.clampOffset()
}

// Filtering reports whether the list is in inline-filter mode, so a wizard can
// avoid intercepting typed keys (e.g. the refresh key) as commands.
func (m SelectListModel) Filtering() bool { return m.filtering }

// Init satisfies tea.Model.
func (m SelectListModel) Init() tea.Cmd { return nil }

// Update handles key events for navigation, filtering, and selection.
func (m SelectListModel) Update(msg tea.Msg) (SelectListModel, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	if m.filtering {
		m = m.updateFilter(keyMsg)
	} else {
		m = m.updateNormal(keyMsg)
	}

	m.clampOffset()
	return m, nil
}

func (m SelectListModel) updateNormal(msg tea.KeyMsg) SelectListModel {
	switch msg.String() {
	case "up", "k":
		m.moveUp()
	case "down", "j":
		m.moveDown()
	case "enter":
		if len(m.filtered) > 0 {
			m.chosen = true
		}
	case "esc":
		if m.filter != "" {
			m.filter = ""
			m.filtering = false
			m.refilter()
			m.snapToSelectable()
			return m
		}
		m.aborted = true
	case "/":
		m.filtering = true
	}
	return m
}

func (m SelectListModel) updateFilter(msg tea.KeyMsg) SelectListModel {
	switch msg.String() {
	case "esc":
		m.filtering = false
	case "backspace":
		if len(m.filter) > 0 {
			m.filter = m.filter[:len(m.filter)-1]
		}
		if m.filter == "" {
			m.filtering = false
		}
		m.refilter()
		m.snapToSelectable()
	case "enter":
		if len(m.filtered) > 0 {
			m.filtering = false
			m.chosen = true
		}
	case "up":
		m.moveUp()
	case "down":
		m.moveDown()
	default:
		if len(msg.String()) == 1 && msg.String() >= " " {
			m.filter += msg.String()
			m.refilter()
			m.snapToSelectable()
		}
	}
	return m
}

// View renders the list with full-row highlight on the selected item. A list
// with no item at all renders nothing: it is a body still being filled (a step
// loading behind a spinner), not a filter that matched nothing.
func (m SelectListModel) View() string {
	if len(m.items) == 0 {
		return ""
	}

	var b strings.Builder

	b.WriteString(renderFilterPrompt(filterPromptParams{query: m.filter, active: m.filtering}))

	visibleHeight := m.visibleHeight()
	if visibleHeight <= 0 {
		visibleHeight = len(m.filtered)
	}

	end := m.offset + visibleHeight
	if end > len(m.filtered) {
		end = len(m.filtered)
	}

	spacer := "\n"

	for vi := m.offset; vi < end; vi++ {
		idx := m.filtered[vi]
		item := m.items[idx]
		selected := vi == m.cursor

		if item.Separator {
			b.WriteString(m.renderSeparator())
		} else {
			b.WriteString(m.renderItem(itemRenderParams{item: item, selected: selected}))
		}
		if vi < end-1 {
			b.WriteString(spacer)
		}
	}

	if len(m.filtered) == 0 {
		b.WriteString(styles.Muted.Render("  No matches"))
	}

	return b.String()
}

// Row layout constants. A row is laid out as:
//
//	<prefix><lead><label><fill><tags><tagGap><status pill>
//
// The label is left-aligned; the tag + status cluster is right-aligned to the
// row width so the status pills form a clean column and the tags hug them. The
// prefix is the 2-cell selection marker (▌▸) or two blank spaces so labels start
// at the same column whether or not the row is selected.
const (
	rowPrefixWidth = 2
	rowLead        = 1
	rowTagGap      = 2
)

// itemRenderParams holds the inputs for rendering a single non-separator row.
type itemRenderParams struct {
	item     SelectItem
	selected bool
}

func (m SelectListModel) renderItem(p itemRenderParams) string {
	statusStr := ""
	statusWidth := 0
	if p.item.Status != nil {
		statusStr = p.item.Status.RenderPill()
		statusWidth = PrintableWidth(statusStr)
	}

	blockWidth := m.width - rowPrefixWidth - statusWidth
	if blockWidth < 1 {
		blockWidth = 1
	}

	if p.selected {
		// Plain (ANSI-free) content so the tint fills every cell; the colored
		// status pill is appended outside the tinted span.
		block := composeRowBlock(rowBlockParams{
			label:      p.item.Label,
			labelWidth: PrintableWidth(p.item.Label),
			tags:       m.renderTagsPlain(p.item.Badges),
			tagsWidth:  m.tagsWidth(p.item.Badges),
			blockWidth: blockWidth,
		})
		return styles.SelectedMarker.Render("▌▸") + styles.ListItemTinted.Render(block) + statusStr
	}

	label := p.item.Label
	switch {
	case p.item.Disabled:
		label = styles.Muted.Render(label)
	case p.item.Danger:
		label = styles.DangerText.Render(label)
	}
	block := composeRowBlock(rowBlockParams{
		label:      label,
		labelWidth: PrintableWidth(p.item.Label),
		tags:       m.renderTagsStyled(p.item.Badges),
		tagsWidth:  m.tagsWidth(p.item.Badges),
		blockWidth: blockWidth,
	})
	return strings.Repeat(" ", rowPrefixWidth) + block + statusStr
}

// rowBlockParams holds the inputs for composeRowBlock.
type rowBlockParams struct {
	label      string // already styled (normal) or raw (selected, wrapped in tint)
	labelWidth int    // printable width of the raw label
	tags       string // rendered tags (styled or plain)
	tagsWidth  int
	blockWidth int
}

// composeRowBlock builds the fixed-width block between the prefix and the status
// pill: a lead space, the left-aligned label, filler, then the right-aligned tag
// cluster followed by a gap so the cluster sits flush against the status pill.
func composeRowBlock(p rowBlockParams) string {
	var b strings.Builder
	b.WriteString(strings.Repeat(" ", rowLead))
	b.WriteString(p.label)

	// Right-align the tags + trailing gap within the remaining width so they end
	// flush against the status pill appended after the block.
	cluster := p.tagsWidth + rowTagGap
	fill := p.blockWidth - rowLead - p.labelWidth - cluster
	if fill < 1 {
		fill = 1
	}
	b.WriteString(strings.Repeat(" ", fill))
	b.WriteString(p.tags)
	b.WriteString(strings.Repeat(" ", rowTagGap))
	return b.String()
}

func (m SelectListModel) renderTagsStyled(badges []Badge) string {
	if len(badges) == 0 {
		return ""
	}
	parts := make([]string, len(badges))
	for i, badge := range badges {
		parts[i] = badge.Render()
	}
	return strings.Join(parts, strings.Repeat(" ", rowTagGap))
}

func (m SelectListModel) renderTagsPlain(badges []Badge) string {
	if len(badges) == 0 {
		return ""
	}
	parts := make([]string, len(badges))
	for i, badge := range badges {
		parts[i] = badge.Text
	}
	return strings.Join(parts, strings.Repeat(" ", rowTagGap))
}

// tagsWidth returns the printable width of the tag column (styled and plain
// renderings share the same visible width).
func (m SelectListModel) tagsWidth(badges []Badge) int {
	return PrintableWidth(m.renderTagsPlain(badges))
}

func (m SelectListModel) renderSeparator() string {
	width := m.width - 4
	if width < 4 {
		width = 4
	}
	return styles.Divider.Render(styles.Indent + strings.Repeat("─", width))
}

func (m *SelectListModel) refilter() {
	m.filtered = filterVisible(filterMatchParams{
		query:    m.filter,
		count:    len(m.items),
		label:    func(i int) string { return m.items[i].Label },
		eligible: func(i int) bool { return !m.items[i].Separator },
	})
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}

func (m *SelectListModel) snapToSelectable() {
	if len(m.filtered) == 0 {
		return
	}
	if !m.isSelectable(m.cursor) {
		m.moveDown()
	}
	if !m.isSelectable(m.cursor) {
		m.moveUp()
	}
}

func (m *SelectListModel) moveUp() {
	for i := m.cursor - 1; i >= 0; i-- {
		if m.isSelectable(i) {
			m.cursor = i
			return
		}
	}
}

func (m *SelectListModel) moveDown() {
	for i := m.cursor + 1; i < len(m.filtered); i++ {
		if m.isSelectable(i) {
			m.cursor = i
			return
		}
	}
}

func (m SelectListModel) isSelectable(idx int) bool {
	if idx < 0 || idx >= len(m.filtered) {
		return false
	}
	item := m.items[m.filtered[idx]]
	return !item.Separator && !item.Disabled
}

func (m SelectListModel) visibleHeight() int {
	if m.height <= 0 {
		return 0
	}
	return max(1, m.height-filterOverhead(m.filtering, m.filter))
}

func (m SelectListModel) helpActions() []string { return []string{domain.HelpFilter} }

func (m SelectListModel) helpModal() string {
	if m.filtering {
		return m.filterHelpHint()
	}
	return ""
}

// filterHelpHint is also read by the standalone list outside the wizard, which
// has no help bar of its own to compose.
func (m SelectListModel) filterHelpHint() string {
	return domain.FilterSelectHelp
}

func (m *SelectListModel) clampOffset() {
	visibleHeight := m.visibleHeight()
	if visibleHeight <= 0 {
		return
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+visibleHeight {
		m.offset = m.cursor - visibleHeight + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
}
