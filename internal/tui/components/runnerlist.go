package components

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
)

// RunnerListModel attaches each service to the root-level one that starts it.
// The answers sit on the row and cycle under ←→, the way the kind step names a
// type: what the reader is choosing between is visible without being remembered.
//
// A row can hold several runners — run.toml allows two roots to start the same
// app — but it is set one at a time: cycling replaces what the row held. A row
// left alone keeps every runner it arrived with, which is what stops a re-init
// from flattening a relation written by hand.
type RunnerListModel struct {
	choices []domain.JobRunnerChoice
	cursor  int
	width   int
	height  int
	title   string
	desc    string
	done    bool
	aborted bool
}

type NewRunnerListParams struct {
	Title       string
	Description string
	Choices     []domain.JobRunnerChoice
}

func NewRunnerList(params NewRunnerListParams) RunnerListModel {
	return RunnerListModel{
		choices: params.Choices,
		title:   params.Title,
		desc:    params.Description,
		width:   80,
	}
}

func (m RunnerListModel) Choices() []domain.JobRunnerChoice { return m.choices }
func (m RunnerListModel) Done() bool                        { return m.done }
func (m RunnerListModel) Aborted() bool                     { return m.aborted }
func (m RunnerListModel) Init() tea.Cmd                     { return nil }

func (m *RunnerListModel) SetSize(params SetSizeParams) {
	m.width, m.height = params.Width, params.Height
}

func (m RunnerListModel) doneRow() int { return len(m.choices) }

func (m RunnerListModel) Update(msg tea.Msg) (RunnerListModel, tea.Cmd) {
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
	case "left", "h":
		return m.cycle(-1), nil
	case "right", "l", " ":
		return m.cycle(1), nil
	case "enter":
		if m.cursor == m.doneRow() {
			m.done = true
			return m, nil
		}
		return m.cycle(1), nil
	case "esc":
		m.aborted = true
	}

	return m, nil
}

func (m RunnerListModel) cycle(step int) RunnerListModel {
	if m.cursor >= len(m.choices) {
		return m
	}

	row := m.choices[m.cursor]
	at := 0
	// A row holding several starts its cycle from the first: whichever it lands
	// on replaces the set, and that is the reader's own gesture.
	for i, option := range row.Options {
		if len(row.Runners) > 0 && option == row.Runners[0] {
			at = i
			break
		}
	}

	choices := make([]domain.JobRunnerChoice, len(m.choices))
	copy(choices, m.choices)
	choices[m.cursor].Runners = nil
	if next := row.Options[((at+step)%len(row.Options)+len(row.Options))%len(row.Options)]; next != "" {
		choices[m.cursor].Runners = []string{next}
	}
	m.choices = choices
	return m
}

func (m RunnerListModel) View() string {
	column := m.runnerColumn()

	var b strings.Builder
	for i, choice := range m.choices {
		m.renderRow(&b, choice, i == m.cursor, column)
		b.WriteString("\n")
	}
	m.renderDoneRow(&b, m.cursor == m.doneRow())
	return b.String()
}

// runnerColumn is where the answer starts on every row: just past the longest
// label, not at the right edge.
func (m RunnerListModel) runnerColumn() int {
	longest := 0
	for _, choice := range m.choices {
		if w := PrintableWidth(choice.Label); w > longest {
			longest = w
		}
	}
	return PrintableWidth(styles.Indent) + longest + domain.RunnerListGap
}

func (m RunnerListModel) helpActions() []string { return []string{domain.HelpSetRunner} }

func (m RunnerListModel) helpModal() string { return "" }

func (m RunnerListModel) renderRow(b *strings.Builder, choice domain.JobRunnerChoice, selected bool, column int) {
	label := choice.Label
	runner := strings.Join(choice.Runners, domain.RunnerListSep)
	if runner == "" {
		runner = domain.RunnerListNone
	}

	line := styles.Indent + label
	if pad := column - PrintableWidth(line); pad > 0 {
		line += strings.Repeat(" ", pad)
	}
	line += runner

	if selected {
		if pad := m.width - PrintableWidth("▸ "+line[len(styles.Indent):]); pad > 0 {
			line = "▸ " + line[len(styles.Indent):] + strings.Repeat(" ", pad)
		}
		b.WriteString(styles.ListItemSelected.Render(line))
		return
	}
	b.WriteString(styles.ListItemNormal.Render(line))
}

func (m RunnerListModel) renderDoneRow(b *strings.Builder, selected bool) {
	if selected {
		line := "▸ " + domain.WizardDoneRow
		if pad := m.width - PrintableWidth(line); pad > 0 {
			line += strings.Repeat(" ", pad)
		}
		b.WriteString(styles.ListItemSelected.Render(line))
		return
	}
	b.WriteString(styles.ListItemNormal.Render(styles.Indent + domain.WizardDoneRow))
}
