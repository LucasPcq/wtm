package components

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
)

// ScopeListModel asks, service by service, which of them run once for the whole
// repository. A service the answer does not belong to — one built from this
// worktree's source — is shown with its reason rather than hidden: the list has
// to be complete, or a re-init would silently drop what a reader answered for.
type ScopeListModel struct {
	entries []rules.ServiceScopeChoice
	cursor  int
	offset  int
	width   int
	height  int
	title   string
	desc    string
	done    bool
	aborted bool
}

type NewScopeListParams struct {
	Title       string
	Description string
	Entries     []rules.ServiceScopeChoice
}

func NewScopeList(params NewScopeListParams) ScopeListModel {
	model := ScopeListModel{
		entries: params.Entries,
		title:   params.Title,
		desc:    params.Description,
		width:   80,
	}
	model.cursor = model.nextAnswerable(0, 1)
	return model
}

func (m ScopeListModel) Entries() []rules.ServiceScopeChoice { return m.entries }
func (m ScopeListModel) Done() bool                          { return m.done }
func (m ScopeListModel) Aborted() bool                       { return m.aborted }
func (m ScopeListModel) Init() tea.Cmd                       { return nil }

func (m *ScopeListModel) SetSize(params SetSizeParams) {
	m.width, m.height = params.Width, params.Height
}

func (m ScopeListModel) Update(msg tea.Msg) (ScopeListModel, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch keyMsg.String() {
	case "up", "k":
		m.cursor = m.nextAnswerable(m.cursor-1, -1)
	case "down", "j":
		m.cursor = m.nextAnswerable(m.cursor+1, 1)
	case "left", "h":
		return m.setScope(domain.JobScopePerWorktree), nil
	case "right", "l":
		return m.setScope(domain.JobScopeShared), nil
	case " ":
		return m.setScope(m.other()), nil
	case "enter":
		m.done = true
	case "esc":
		m.aborted = true
	}

	return m.scrolled(), nil
}

// nextAnswerable walks past the rows whose answer is not the reader's, in the
// given direction, and stays put rather than landing on one.
func (m ScopeListModel) nextAnswerable(from, direction int) int {
	for cursor := from; cursor >= 0 && cursor < len(m.entries); cursor += direction {
		if !m.entries[cursor].Fixed {
			return cursor
		}
	}
	return m.cursor
}

func (m ScopeListModel) other() domain.JobScope {
	if m.cursor >= len(m.entries) || m.entries[m.cursor].Scope == domain.JobScopeShared {
		return domain.JobScopePerWorktree
	}
	return domain.JobScopeShared
}

func (m ScopeListModel) setScope(scope domain.JobScope) ScopeListModel {
	if m.cursor >= len(m.entries) || m.entries[m.cursor].Fixed {
		return m
	}
	m.entries[m.cursor].Scope = scope
	return m
}

func (m ScopeListModel) View() string {
	body, _ := windowBody(m.window())
	return body
}

func (m ScopeListModel) window() bodyWindowParams {
	column := m.scopeColumn()
	rows := make([]string, 0, len(m.entries))
	for i, entry := range m.entries {
		rows = append(rows, m.renderRow(entry, i == m.cursor, column))
	}
	return bodyWindowParams{
		Rows: rows, Offset: m.offset, Height: m.height,
		Top: m.cursor, Bottom: m.cursor,
	}
}

// scrolled settles where the window sits after the cursor moved, so the next
// render scrolls from there rather than snapping back to the top of the list.
func (m ScopeListModel) scrolled() ScopeListModel {
	_, m.offset = windowBody(m.window())
	return m
}

func (m ScopeListModel) scopeColumn() int {
	longest := 0
	for _, entry := range m.entries {
		if w := PrintableWidth(scopeLabel(entry)); w > longest {
			longest = w
		}
	}
	return PrintableWidth(styles.Indent) + longest + domain.KindListGap
}

func (m ScopeListModel) helpActions() []string { return []string{domain.HelpSetScope} }

func (m ScopeListModel) helpModal() string { return "" }

func (m ScopeListModel) renderRow(entry rules.ServiceScopeChoice, selected bool, column int) string {
	prefix := styles.Indent
	if selected {
		prefix = "▸ "
	}
	left := prefix + scopeLabel(entry)

	gap := column - PrintableWidth(left)
	if gap < domain.KindListGap {
		gap = domain.KindListGap
	}
	line := left + strings.Repeat(" ", gap) + scopeAnswer(entry)

	if entry.Fixed {
		return styles.Muted.Render(line)
	}
	if !selected {
		return styles.ListItemNormal.Render(line)
	}
	if pad := m.width - PrintableWidth(line); pad > 0 {
		line += strings.Repeat(" ", pad)
	}
	return styles.ListItemSelected.Render(line)
}

func scopeLabel(entry rules.ServiceScopeChoice) string {
	image := entry.Image
	if image == "" {
		image = entry.File
	}
	return fmt.Sprintf(domain.ScopeListEntryFmt, entry.Service, image)
}

func scopeAnswer(entry rules.ServiceScopeChoice) string {
	if entry.Fixed {
		return fmt.Sprintf(domain.ScopeListFixedFmt, entry.Reason)
	}
	per, shared := domain.KindRadioOn, domain.KindRadioOff
	if entry.Scope == domain.JobScopeShared {
		per, shared = domain.KindRadioOff, domain.KindRadioOn
	}
	return fmt.Sprintf(domain.ScopeListRadiosFmt, per, shared)
}
