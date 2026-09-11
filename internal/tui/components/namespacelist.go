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

// NamespaceListModel asks, for each shared service, what each worktree gets of
// it. Three lines per service — its name, the command that creates it, the one
// that gives it back — because wtm has nothing to propose for the last two: it
// knows the variables a command may read, never what a database or a realm is.
type NamespaceListModel struct {
	fields  []domain.NamespaceField
	cursor  int
	offset  int
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

type NewNamespaceListParams struct {
	Title       string
	Description string
	Fields      []domain.NamespaceField
}

func NewNamespaceList(params NewNamespaceListParams) NamespaceListModel {
	return NamespaceListModel{
		fields: params.Fields,
		title:  params.Title,
		desc:   params.Description,
		width:  80,
	}
}

func (m NamespaceListModel) Fields() []domain.NamespaceField { return m.fields }
func (m NamespaceListModel) Done() bool                      { return m.done }
func (m NamespaceListModel) Aborted() bool                   { return m.aborted }
func (m NamespaceListModel) Init() tea.Cmd                   { return nil }

func (m *NamespaceListModel) SetSize(params SetSizeParams) {
	m.width, m.height = params.Width, params.Height
}

func (m NamespaceListModel) doneRow() int { return len(m.fields) }

func (m NamespaceListModel) Update(msg tea.Msg) (NamespaceListModel, tea.Cmd) {
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
	case "enter":
		if m.cursor == m.doneRow() {
			m.done = true
			return m, nil
		}
		return m.startEdit(), nil
	case "esc":
		m.aborted = true
	}

	return m.scrolled(), nil
}

// startEdit opens on what is already there: a command is amended, not retyped.
func (m NamespaceListModel) startEdit() NamespaceListModel {
	input := textinput.New()
	input.CharLimit = domain.CmdListCharLimit
	input.Width = max(domain.CmdListMinWidth, m.width-domain.CmdListWidthInset)
	input.SetValue(m.fields[m.cursor].Value)
	input.CursorEnd()
	input.Focus()

	m.input = input
	m.editing = true
	m.err = ""
	return m
}

func (m NamespaceListModel) updateEdit(msg tea.Msg) (NamespaceListModel, tea.Cmd) {
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

// saveEdit refuses only an empty name. A create left blank is an answer — the
// service is shared outright, data included — and a remove left blank means the
// slice is never given back, which clean reports rather than refuses.
func (m NamespaceListModel) saveEdit() NamespaceListModel {
	value := strings.TrimSpace(m.input.Value())
	if value == "" && m.fields[m.cursor].Field == domain.NamespaceFieldName {
		m.err = domain.NamespaceNameEmptyErr
		return m
	}

	m.fields[m.cursor].Value = value
	m.editing = false
	m.err = ""
	return m
}

func (m NamespaceListModel) View() string {
	body, _ := windowBody(m.window())
	return body
}

func (m NamespaceListModel) window() bodyWindowParams {
	jobWidth := rules.NamespaceJobWidth(m.fields)

	rows := make([]string, 0, len(m.fields)+1)
	for i, field := range m.fields {
		// The job is named once per service, not on each of its three lines: it
		// is one question in three parts, and repeating the name made six rows
		// read as six unrelated ones.
		heads := i == 0 || m.fields[i-1].Job != field.Job
		rows = append(rows, m.renderRow(namespaceRowParams{
			Field: field, JobWidth: jobWidth, Heads: heads, Editing: m.editing && i == m.cursor,
		}, i == m.cursor))
	}
	rows = append(rows, m.renderRow(namespaceRowParams{Done: true}, m.cursor == m.doneRow()))

	var b strings.Builder
	// The variables are shown while typing, where they are needed, and they are
	// this job's own: the ports it declares under the names it declares them by.
	if m.editing {
		b.WriteString(renderVarGroups(renderVarGroupsParams{
			Groups: m.fields[m.cursor].Vars, Width: m.width,
		}))
	}
	if m.err != "" {
		b.WriteString("\n\n")
		b.WriteString(errorBanner(m.err))
	}

	return bodyWindowParams{
		Rows: rows, Offset: m.offset, Height: m.height,
		Top: m.jobHead(m.cursor), Bottom: m.cursor, Extras: b.String(),
	}
}

// jobHead is the row naming the job the cursor sits in: a scroll that left it
// behind would show three unlabelled lines belonging to nothing.
func (m NamespaceListModel) jobHead(cursor int) int {
	if cursor >= len(m.fields) {
		return cursor
	}
	head := cursor
	for head > 0 && m.fields[head-1].Job == m.fields[cursor].Job {
		head--
	}
	return head
}

// scrolled settles where the window sits after the cursor moved, so the next
// render scrolls from there rather than snapping back to the top of the list.
func (m NamespaceListModel) scrolled() NamespaceListModel {
	_, m.offset = windowBody(m.window())
	return m
}

func (m NamespaceListModel) helpActions() []string { return nil }

func (m NamespaceListModel) helpModal() string {
	if m.editing {
		return domain.NamespaceEditHelp
	}
	return ""
}

type namespaceRowParams struct {
	Field    domain.NamespaceField
	JobWidth int
	// Heads says this row is the first of its service, and so the one that
	// carries its name.
	Heads   bool
	Editing bool
	Done    bool
}

func (m NamespaceListModel) renderRow(params namespaceRowParams, selected bool) string {
	label := domain.WizardDoneRow
	if !params.Done {
		value := params.Field.Value
		if params.Editing {
			value = m.input.View()
		} else if value == "" {
			value = styles.Muted.Render(domain.NamespaceEmptyValue)
		}
		job := params.Field.Job
		if !params.Heads {
			job = ""
		}
		label = fmt.Sprintf(domain.NamespaceRowFmt, params.JobWidth, job, string(params.Field.Field), value)
	}

	if selected {
		line := "▸ " + label
		if pad := m.width - PrintableWidth(line); pad > 0 {
			line += strings.Repeat(" ", pad)
		}
		return styles.ListItemSelected.Render(line)
	}
	return styles.ListItemNormal.Render(styles.Indent + label)
}
