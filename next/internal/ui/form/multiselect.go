package form

import (
	"errors"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/ui/output"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	pickFilterHelp = "  type to filter • space toggle • enter confirm • esc back"
	helpPick       = "space toggle"
	helpPickAll    = "a all"
)

type Tone int

const (
	ToneMuted Tone = iota
	ToneWarning
	ToneDanger
)

type Pick struct {
	Label  string
	Value  string
	Tag    string
	Tone   Tone
	Picked bool
}

type MultiSelect struct {
	options   []Pick
	shown     []int
	cursor    int
	offset    int
	query     string
	filtering bool
	empty     string
	err       error
}

type MultiSelectParams struct {
	Options []Pick
	Empty   string
}

func NewMultiSelect(params MultiSelectParams) *MultiSelect {
	m := &MultiSelect{options: append([]Pick(nil), params.Options...), empty: params.Empty}
	m.refilter()
	return m
}

func (m *MultiSelect) Values() []string {
	var values []string
	for _, o := range m.options {
		if o.Picked {
			values = append(values, o.Value)
		}
	}
	return values
}

func (m *MultiSelect) Update(msg tea.Msg) (Verdict, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return Stay, nil
	}
	if m.filtering {
		return m.filter(key), nil
	}
	switch key.String() {
	case "up", "k":
		m.move(-1)
	case "down", "j":
		m.move(1)
	case " ":
		m.toggle()
	case "a":
		m.pickAll()
	case "enter":
		return m.confirm(), nil
	case "esc":
		if m.query == "" {
			return Back, nil
		}
		m.query = ""
		m.refilter()
	case "/":
		m.filtering = true
	}
	return Stay, nil
}

func (m *MultiSelect) filter(key tea.KeyMsg) Verdict {
	switch typed := key.String(); typed {
	case "esc":
		m.filtering = false
	case "backspace":
		m.query = m.query[:max(len(m.query)-1, 0)]
		m.filtering = m.query != ""
		m.refilter()
	case "enter":
		m.filtering = false
		return m.confirm()
	case " ":
		m.toggle()
	case "up":
		m.move(-1)
	case "down":
		m.move(1)
	default:
		if len(typed) == 1 && typed > " " {
			m.query += typed
			m.refilter()
		}
	}
	return Stay
}

func (m *MultiSelect) toggle() {
	if m.cursor >= len(m.shown) {
		return
	}
	at := m.shown[m.cursor]
	m.options[at].Picked = !m.options[at].Picked
	m.err = nil
}

func (m *MultiSelect) pickAll() {
	for _, at := range m.shown {
		m.options[at].Picked = true
	}
	m.err = nil
}

func (m *MultiSelect) confirm() Verdict {
	if len(m.Values()) == 0 {
		m.err = errors.New(m.empty)
		return Stay
	}
	return Done
}

func (m *MultiSelect) refilter() {
	m.shown = m.shown[:0]
	query := strings.ToLower(m.query)
	for i, o := range m.options {
		if strings.Contains(strings.ToLower(o.Label), query) {
			m.shown = append(m.shown, i)
		}
	}
	m.cursor = min(m.cursor, max(len(m.shown)-1, 0))
}

func (m *MultiSelect) move(step int) {
	m.cursor = min(max(m.cursor+step, 0), max(len(m.shown)-1, 0))
}

func (m *MultiSelect) View(size Size, st *Styles) string {
	var b strings.Builder
	if m.filtering || m.query != "" {
		b.WriteString(st.Muted.Render("/ " + m.query))
		if m.filtering {
			b.WriteString(st.reverse.Render(" "))
		}
		b.WriteString("\n\n")
	}
	rows := len(m.shown)
	if size.Height > 0 {
		rows = max(1, size.Height-m.overhead())
	}
	m.offset = min(max(m.offset, m.cursor-rows+1), m.cursor)
	end := min(m.offset+rows, len(m.shown))
	for i := m.offset; i < end; i++ {
		b.WriteString(m.row(m.options[m.shown[i]], i == m.cursor, size.Width, st))
		if i < end-1 {
			b.WriteString("\n")
		}
	}
	if len(m.shown) == 0 {
		b.WriteString(st.Muted.Render(output.Indent + noMatches))
	}
	if m.err != nil {
		b.WriteString("\n\n" + errorBanner(m.err.Error(), size.Width, st))
	}
	return b.String()
}

func (m *MultiSelect) overhead() int {
	lines := 0
	if m.filtering || m.query != "" {
		lines += 2
	}
	if m.err != nil {
		lines += 2
	}
	return lines
}

func (m *MultiSelect) row(p Pick, selected bool, width int, st *Styles) string {
	check := checkOff
	if p.Picked {
		check = checkOn
	}
	label := check + " " + p.Label
	fill := max(width-rowPrefix-rowLead-lipgloss.Width(label)-lipgloss.Width(p.Tag)-rowTagGap, 1)
	if selected {
		block := " " + label + strings.Repeat(" ", fill) + p.Tag + strings.Repeat(" ", rowTagGap)
		return st.accent.Render("▌›") + st.tinted.Render(block)
	}
	return strings.Repeat(" ", rowPrefix+rowLead) + label + strings.Repeat(" ", fill) + toned(p.Tone, st).Render(p.Tag) + strings.Repeat(" ", rowTagGap)
}

func toned(t Tone, st *Styles) lipgloss.Style {
	switch t {
	case ToneWarning:
		return st.Warning
	case ToneDanger:
		return st.Danger
	case ToneMuted:
	}
	return st.Muted
}

func (m *MultiSelect) Help() Help {
	if m.filtering {
		return Help{Modal: pickFilterHelp}
	}
	return Help{Actions: []string{helpPick, helpPickAll, helpFilter}}
}
