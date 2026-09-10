package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
)

// EnvValueListModel asks which .env keys name each worktree's slice of a shared
// service. Every managed key is a row because wtm cannot recognize a realm name
// — the value is opaque — so the list is complete and the reader points. The
// value each key holds today is shown beside it, since that is the only thing
// that lets one be told from another.
type EnvValueListModel struct {
	fields  []domain.EnvValueField
	cursor  int
	width   int
	height  int
	title   string
	desc    string
	done    bool
	aborted bool

	editing bool
	input   textinput.Model
	err     string
}

type NewEnvValueListParams struct {
	Title       string
	Description string
	Fields      []domain.EnvValueField
}

func NewEnvValueList(params NewEnvValueListParams) EnvValueListModel {
	return EnvValueListModel{
		fields: params.Fields,
		title:  params.Title,
		desc:   params.Description,
		width:  80,
	}
}

func (m EnvValueListModel) Fields() []domain.EnvValueField { return m.fields }
func (m EnvValueListModel) Done() bool                     { return m.done }
func (m EnvValueListModel) Aborted() bool                  { return m.aborted }
func (m EnvValueListModel) Init() tea.Cmd                  { return nil }
func (m EnvValueListModel) Title() string                  { return m.title }
func (m EnvValueListModel) Description() string            { return m.desc }

func (m *EnvValueListModel) SetSize(params SetSizeParams) {
	m.width, m.height = params.Width, params.Height
}

func (m EnvValueListModel) doneRow() int { return len(m.fields) }

func (m EnvValueListModel) Update(msg tea.Msg) (EnvValueListModel, tea.Cmd) {
	if m.editing {
		return m.updateEdit(msg)
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch keyMsg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < m.doneRow() {
			m.cursor++
		}
	case " ":
		return m.toggle(), nil
	case "enter":
		if m.cursor == m.doneRow() {
			m.done = true
			return m, nil
		}
		return m.startEdit(), nil
	case "esc":
		m.aborted = true
	}

	return m, nil
}

func (m EnvValueListModel) toggle() EnvValueListModel {
	if m.cursor >= len(m.fields) {
		return m
	}
	m.fields[m.cursor].Linked = !m.fields[m.cursor].Linked
	return m
}

// startEdit opens on the template already there, and links the row: editing a
// template is asking for it to be written.
func (m EnvValueListModel) startEdit() EnvValueListModel {
	input := textinput.New()
	input.CharLimit = domain.CmdListCharLimit
	input.Width = max(domain.CmdListMinWidth, m.width-domain.CmdListWidthInset)
	input.SetValue(m.fields[m.cursor].Value)
	input.CursorEnd()
	input.Focus()

	m.fields[m.cursor].Linked = true
	m.input = input
	m.editing = true
	m.err = ""
	return m
}

func (m EnvValueListModel) updateEdit(msg tea.Msg) (EnvValueListModel, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	switch keyMsg.String() {
	case "enter":
		return m.saveEdit(), nil
	case "esc":
		m.editing = false
		m.err = ""
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// saveEdit refuses an empty template on a linked row: wtm would own the line and
// write nothing into it. Unlinking the row is how a key is left alone.
func (m EnvValueListModel) saveEdit() EnvValueListModel {
	value := strings.TrimSpace(m.input.Value())
	if value == "" {
		m.err = domain.EnvValueEmptyErr
		return m
	}

	m.fields[m.cursor].Value = value
	m.editing = false
	m.err = ""
	return m
}

func (m EnvValueListModel) View() string {
	keyWidth := rules.EnvValueKeyWidth(m.fields)

	var b strings.Builder
	for i, field := range m.fields {
		// The service and the file are named once per group, not on each row:
		// repeating them turned thirty keys into thirty unrelated lines.
		if i == 0 || m.fields[i-1].Job != field.Job || m.fields[i-1].File != field.File {
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString(styles.Indent)
			b.WriteString(styles.Muted.Render(fmt.Sprintf(domain.EnvValueGroupFmt, field.Job, field.File)))
			b.WriteString("\n")
		}
		m.renderRow(&b, envValueRowParams{
			Field: field, KeyWidth: keyWidth, Editing: m.editing && i == m.cursor,
		}, i == m.cursor)
		b.WriteString("\n")
	}
	m.renderRow(&b, envValueRowParams{Done: true}, m.cursor == m.doneRow())

	if m.editing {
		b.WriteString(renderVarGroups(renderVarGroupsParams{
			Groups: m.fields[m.cursor].Vars, Width: m.width,
		}))
	}
	if m.err != "" {
		b.WriteString("\n\n")
		b.WriteString(errorBanner(m.err))
	}
	return b.String()
}

func (m EnvValueListModel) helpActions() []string { return []string{domain.EnvValueHelpLink} }

func (m EnvValueListModel) helpModal() string {
	if m.editing {
		return domain.EnvValueEditHelp
	}
	return ""
}

type envValueRowParams struct {
	Field    domain.EnvValueField
	KeyWidth int
	Editing  bool
	Done     bool
}

func (m EnvValueListModel) renderRow(b *strings.Builder, params envValueRowParams, selected bool) {
	label := domain.WizardDoneRow
	if !params.Done {
		label = fmt.Sprintf(domain.EnvValueRowFmt, envValueMark(params.Field),
			params.KeyWidth, params.Field.Key, m.rowValue(params))
	}

	if selected {
		line := "▸ " + label
		if pad := m.width - PrintableWidth(line); pad > 0 {
			line += strings.Repeat(" ", pad)
		}
		b.WriteString(styles.ListItemSelected.Render(line))
		return
	}
	b.WriteString(styles.ListItemNormal.Render(styles.Indent + label))
}

// rowValue is the template when the key is wtm's, and what the file holds today
// when it is not: an unlinked row shows the reader what the key is, a linked one
// shows what it becomes.
func (m EnvValueListModel) rowValue(params envValueRowParams) string {
	if params.Editing {
		return m.input.View()
	}
	if !params.Field.Linked {
		current := params.Field.Current
		if current == "" {
			current = domain.EnvValueEmptyValue
		}
		return styles.Muted.Render(current)
	}
	value := params.Field.Value
	if value == "" {
		value = styles.Muted.Render(domain.EnvValueEmptyValue)
	}
	if params.Field.Current == "" {
		return value
	}
	return value + styles.Muted.Render(fmt.Sprintf(domain.EnvValueCurrentFmt, params.Field.Current))
}

func envValueMark(field domain.EnvValueField) string {
	if field.Linked {
		return domain.EnvValueMarkOn
	}
	return domain.EnvValueMarkOff
}
