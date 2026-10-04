package components

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
)

// TagVariant selects the color of a MultiSelectItem's status tag.
type TagVariant int

const (
	// TagNeutral renders the tag in the muted color.
	TagNeutral TagVariant = iota
	// TagSuccess renders the tag in green.
	TagSuccess
	// TagWarning renders the tag in yellow.
	TagWarning
	// TagDanger renders the tag in red.
	TagDanger
)

// multiSelectTagWidth is the fixed column width of the optional status tag so
// labels stay aligned regardless of tag length.
const multiSelectTagWidth = 3

// MultiSelectItem represents a toggleable entry in a MultiSelect.
type MultiSelectItem struct {
	Label    string
	Value    string
	Selected bool
	// Tag is an optional short status word shown before the label. It is colored
	// per Variant on normal rows and left plain on the highlighted row.
	Tag     string
	Variant TagVariant
	// Badges are trailing words right-aligned on the row, as on a SelectList.
	Badges []Badge
}

// MultiSelectModel is a checkbox list with space toggle, enter confirm, and
// inline filtering (press "/").
type MultiSelectModel struct {
	items     []MultiSelectItem
	filtered  []int
	cursor    int
	filter    string
	filtering bool
	width     int
	height    int
	offset    int
	title     string
	desc      string
	validate  func([]string) error
	err       error
	done      bool
	aborted   bool
}

// NewMultiSelect creates a MultiSelectModel.
func NewMultiSelect(params NewMultiSelectParams) MultiSelectModel {
	m := MultiSelectModel{
		items:    params.Items,
		title:    params.Title,
		desc:     params.Description,
		validate: params.Validate,
		width:    80,
	}
	m.refilter()
	m.startOn(params.Start)
	return m
}

// startOn places the cursor on the named value, as SelectListModel does.
func (m *MultiSelectModel) startOn(value string) {
	if value == "" {
		return
	}
	for i, idx := range m.filtered {
		if m.items[idx].Value == value {
			m.cursor = i
			return
		}
	}
}

// NewMultiSelectParams holds inputs for NewMultiSelect.
type NewMultiSelectParams struct {
	Title       string
	Description string
	Items       []MultiSelectItem
	// Validate is called on Enter and after each toggle. When it returns an
	// error, Enter does not advance and the message is rendered below the list.
	Validate func([]string) error
	// Start is the value the cursor opens on; empty or unknown leaves it first.
	Start string
}

// Done returns true after the user confirmed.
func (m MultiSelectModel) Done() bool { return m.done }

// Aborted returns true after the user pressed Esc.
func (m MultiSelectModel) Aborted() bool { return m.aborted }

// Values returns the values of all selected items.
func (m MultiSelectModel) Values() []string {
	var vals []string
	for _, item := range m.items {
		if item.Selected {
			vals = append(vals, item.Value)
		}
	}
	return vals
}

// SetSize fits the list to the region its host gives it, as SelectListModel does.
// The wizard reaches the fields directly; a host outside this package cannot.
func (m *MultiSelectModel) SetSize(params SetSizeParams) {
	m.width, m.height = params.Width, params.Height
	m.clampOffset()
}

// Init satisfies tea.Model.
func (m MultiSelectModel) Init() tea.Cmd { return nil }

// Update handles key events for navigation, filtering, toggling, and selection.
func (m MultiSelectModel) Update(msg tea.Msg) (MultiSelectModel, tea.Cmd) {
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

func (m MultiSelectModel) updateNormal(msg tea.KeyMsg) MultiSelectModel {
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
		}
	case " ":
		m.toggleCursor()
		m.refreshValidation()
	case "a":
		m.toggleAll()
		m.refreshValidation()
	case "/":
		m.filtering = true
	case "enter":
		if m.validate != nil {
			if err := m.validate(m.Values()); err != nil {
				m.err = err
				return m
			}
		}
		m.done = true
	case "esc":
		if m.filter != "" {
			m.filter = ""
			m.filtering = false
			m.refilter()
			return m
		}
		m.aborted = true
	}
	return m
}

func (m MultiSelectModel) updateFilter(msg tea.KeyMsg) MultiSelectModel {
	switch msg.String() {
	case "esc", "enter":
		m.filtering = false
	case "backspace":
		if len(m.filter) > 0 {
			m.filter = m.filter[:len(m.filter)-1]
		}
		if m.filter == "" {
			m.filtering = false
		}
		m.refilter()
	case "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down":
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
		}
	default:
		if len(msg.String()) == 1 && msg.String() >= " " {
			m.filter += msg.String()
			m.refilter()
		}
	}
	return m
}

// toggleCursor flips the selection of the item under the cursor.
func (m *MultiSelectModel) toggleCursor() {
	if m.cursor < 0 || m.cursor >= len(m.filtered) {
		return
	}
	idx := m.filtered[m.cursor]
	m.items[idx].Selected = !m.items[idx].Selected
}

// toggleAll selects every filtered item, or clears them when all are already
// selected, so a single key flips the current (filtered) view between "all" and
// "none". Items hidden by the active filter keep their selection.
func (m *MultiSelectModel) toggleAll() {
	allSelected := true
	for _, idx := range m.filtered {
		if !m.items[idx].Selected {
			allSelected = false
			break
		}
	}
	for _, idx := range m.filtered {
		m.items[idx].Selected = !allSelected
	}
}

// refilter recomputes the visible index set from the current filter, matching
// the item Label by case-insensitive substring, and clamps the cursor.
func (m *MultiSelectModel) refilter() {
	m.filtered = filterVisible(filterMatchParams{
		query: m.filter,
		count: len(m.items),
		label: func(i int) string { return m.items[i].Label },
	})
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}

// refreshValidation re-runs validate against the current selection so the
// error clears the moment the user fixes the state.
func (m *MultiSelectModel) refreshValidation() {
	if m.validate == nil {
		return
	}
	if err := m.validate(m.Values()); err != nil {
		m.err = err
	} else {
		m.err = nil
	}
}

// selectedMarkerWidth is what the accent bar takes before the tinted span begins.
// It matches the indent of an unselected row, so the checkboxes stay one column.
const selectedMarkerWidth = 2

// View renders the checkbox list, including the filter prompt when filtering.
func (m MultiSelectModel) View() string {
	if len(m.items) == 0 {
		return styles.Muted.Render("  No items")
	}

	var b strings.Builder

	b.WriteString(renderFilterPrompt(filterPromptParams{
		query:         m.filter,
		active:        m.filtering,
		selectedCount: len(m.Values()),
	}))

	if len(m.filtered) == 0 {
		b.WriteString(styles.Muted.Render("  No matches"))
		return b.String()
	}

	visibleHeight := m.visibleHeight()
	if visibleHeight <= 0 {
		visibleHeight = len(m.filtered)
	}

	end := m.offset + visibleHeight
	if end > len(m.filtered) {
		end = len(m.filtered)
	}

	for vi := m.offset; vi < end; vi++ {
		item := m.items[m.filtered[vi]]
		selected := vi == m.cursor

		check := "[ ]"
		if item.Selected {
			check = "[✓]"
		}

		if selected {
			// The same focus language as SelectListModel: an accent bar and the
			// subtle row tint, not the heavy highlight this list was left on. Plain
			// (ANSI-free) content so the tint fills every cell.
			left := check + " " + plainTag(item.Tag) + item.Label
			badges := plainBadges(item.Badges)
			block := left + m.badgeFill(badgeFillParams{Left: PrintableWidth(left), Badges: PrintableWidth(badges)}) + badges
			if pad := m.width - selectedMarkerWidth - PrintableWidth(block); pad > 0 {
				block += strings.Repeat(" ", pad)
			}
			b.WriteString(styles.SelectedMarker.Render("▌ ") + styles.ListItemTinted.Render(block))
		} else {
			left := check + " " + plainTag(item.Tag) + item.Label
			fill := m.badgeFill(badgeFillParams{Left: PrintableWidth(left), Badges: PrintableWidth(plainBadges(item.Badges))})
			line := styles.Indent + check + " " + coloredTag(item.Tag, item.Variant) + item.Label + fill
			b.WriteString(styles.ListItemNormal.Render(line) + styledBadges(item.Badges))
		}

		if vi < end-1 {
			b.WriteString("\n")
		}
	}

	if m.err != nil {
		b.WriteString("\n\n")
		b.WriteString(wrappedErrorBanner(m.err.Error(), m.width))
	}

	return b.String()
}

type badgeFillParams struct {
	Left   int
	Badges int
}

// badgeFill is the run of spaces that pushes a row's badges against its right
// edge, less the gap SelectListModel keeps there. A row without badges gets
// none.
func (m MultiSelectModel) badgeFill(params badgeFillParams) string {
	if params.Badges == 0 {
		return ""
	}
	return strings.Repeat(" ", max(m.width-selectedMarkerWidth-params.Left-params.Badges-rowTagGap, 1))
}

func plainBadges(badges []Badge) string {
	parts := make([]string, 0, len(badges))
	for _, badge := range badges {
		parts = append(parts, badge.Text)
	}
	return strings.Join(parts, strings.Repeat(" ", rowTagGap))
}

func styledBadges(badges []Badge) string {
	parts := make([]string, 0, len(badges))
	for _, badge := range badges {
		parts = append(parts, badge.Render())
	}
	return strings.Join(parts, strings.Repeat(" ", rowTagGap))
}

// plainTag returns the padded, uncolored tag followed by a space, or an empty
// string when there is no tag. Used on the highlighted row where coloring would
// break the background fill.
func plainTag(tag string) string {
	if tag == "" {
		return ""
	}
	return padTag(tag) + " "
}

// coloredTag returns the padded tag colored per variant followed by a space, or
// an empty string when there is no tag.
func coloredTag(tag string, variant TagVariant) string {
	if tag == "" {
		return ""
	}
	return renderTag(variant, padTag(tag)) + " "
}

func padTag(tag string) string {
	pad := multiSelectTagWidth - PrintableWidth(tag)
	if pad > 0 {
		return tag + strings.Repeat(" ", pad)
	}
	return tag
}

func renderTag(variant TagVariant, tag string) string {
	switch variant {
	case TagSuccess:
		return styles.Success.Render(tag)
	case TagWarning:
		return styles.Warning.Render(tag)
	case TagDanger:
		return styles.DangerText.Render(tag)
	default:
		return styles.Muted.Render(tag)
	}
}

func (m MultiSelectModel) visibleHeight() int {
	if m.height <= 0 {
		return 0
	}
	return max(1, m.height-filterOverhead(m.filtering, m.filter))
}

func (m MultiSelectModel) helpActions() []string {
	return []string{domain.HelpToggle, domain.HelpAll, domain.HelpFilter}
}

func (m MultiSelectModel) helpModal() string {
	if m.filtering {
		return domain.FilterHelp
	}
	return ""
}

func (m *MultiSelectModel) clampOffset() {
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
