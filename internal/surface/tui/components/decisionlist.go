package components

import (
	"fmt"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/styles"
)

const (
	decisionRowFmt  = "%s%s"
	decisionGap     = 3
	decisionCursor  = "› "
	decisionPickFmt = "‹ %s ›"
	helpDecide      = "←→ choose"
)

// DecisionRow is one key to decide: the choice among its options, the first
// when Choice is none of them.
type DecisionRow struct {
	Key     string
	Options []string
	Choice  string
}

type NewDecisionListParams struct {
	Title       string
	Description string
	Rows        []DecisionRow
	// Validate runs on enter; an error keeps the step open and shows under it.
	Validate func(map[string]string) error
}

// DecisionListModel settles one choice per key, each row cycled in place: the
// answer of a decisions field, such as what to do with each .env file.
type DecisionListModel struct {
	rows     []DecisionRow
	validate func(map[string]string) error
	err      error
	cursor   int
	offset   int
	width    int
	height   int
	title    string
	desc     string
	done     bool
	aborted  bool
}

func NewDecisionList(params NewDecisionListParams) DecisionListModel {
	rows := slices.Clone(params.Rows)
	for i, row := range rows {
		if !slices.Contains(row.Options, row.Choice) && len(row.Options) > 0 {
			rows[i].Choice = row.Options[0]
		}
	}
	return DecisionListModel{rows: rows, validate: params.Validate, title: params.Title, desc: params.Description, width: 80}
}

func (m DecisionListModel) Done() bool    { return m.done }
func (m DecisionListModel) Aborted() bool { return m.aborted }
func (m DecisionListModel) Init() tea.Cmd { return nil }

func (m DecisionListModel) Values() map[string]string {
	values := make(map[string]string, len(m.rows))
	for _, row := range m.rows {
		values[row.Key] = row.Choice
	}
	return values
}

func (m DecisionListModel) Update(msg tea.Msg) (DecisionListModel, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch keyMsg.String() {
	case "up", "k":
		m.cursor = max(m.cursor-1, 0)
	case "down", "j":
		m.cursor = min(m.cursor+1, len(m.rows)-1)
	case "left", "h":
		m = m.cycle(-1)
	case "right", "l", " ":
		m = m.cycle(1)
	case "enter":
		return m.confirm(), nil
	case "esc":
		m.aborted = true
	}
	return m.scrolled(), nil
}

func (m DecisionListModel) confirm() DecisionListModel {
	if m.validate != nil {
		if m.err = m.validate(m.Values()); m.err != nil {
			return m
		}
	}
	m.done = true
	return m
}

func (m DecisionListModel) cycle(step int) DecisionListModel {
	if len(m.rows) == 0 {
		return m
	}
	rows := slices.Clone(m.rows)
	row := &rows[m.cursor]
	if len(row.Options) == 0 {
		return m
	}
	at := slices.Index(row.Options, row.Choice)
	row.Choice = row.Options[(at+step+len(row.Options))%len(row.Options)]
	m.rows = rows
	m.err = nil
	return m
}

func (m DecisionListModel) View() string {
	body, _ := windowBody(m.window())
	return body
}

func (m DecisionListModel) window() bodyWindowParams {
	column := m.choiceColumn()
	rows := make([]string, 0, len(m.rows))
	for i, row := range m.rows {
		rows = append(rows, m.renderRow(row, i == m.cursor, column))
	}
	extras := ""
	if m.err != nil {
		extras = "\n\n" + wrappedErrorBanner(m.err.Error(), m.width)
	}
	return bodyWindowParams{Rows: rows, Offset: m.offset, Height: m.height, Top: m.cursor, Bottom: m.cursor, Extras: extras}
}

func (m DecisionListModel) scrolled() DecisionListModel {
	_, m.offset = windowBody(m.window())
	return m
}

// choiceColumn lines the choices up just past the longest key.
func (m DecisionListModel) choiceColumn() int {
	longest := 0
	for _, row := range m.rows {
		longest = max(longest, PrintableWidth(row.Key))
	}
	return PrintableWidth(styles.Indent) + longest + decisionGap
}

func (m DecisionListModel) renderRow(row DecisionRow, selected bool, column int) string {
	prefix := styles.Indent
	if selected {
		prefix = decisionCursor
	}
	left := fmt.Sprintf(decisionRowFmt, prefix, row.Key)
	line := left + strings.Repeat(" ", max(column-PrintableWidth(left), decisionGap)) + fmt.Sprintf(decisionPickFmt, row.Choice)
	if !selected {
		return styles.ListItemNormal.Render(line)
	}
	if pad := m.width - PrintableWidth(line); pad > 0 {
		line += strings.Repeat(" ", pad)
	}
	return styles.ListItemSelected.Render(line)
}

func (m DecisionListModel) helpActions() []string { return []string{helpDecide} }

func (m DecisionListModel) helpModal() string { return "" }
