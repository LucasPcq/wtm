package form

import (
	"strings"

	"github.com/LucasPcq/wtm/next/internal/ui/output"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	rowPrefix   = 2
	rowLead     = 1
	rowTagGap   = 2
	checkOn     = "[x]"
	checkOff    = "[ ]"
	noMatches   = "No matches"
	filterHelp  = "  type to filter • enter select • esc back"
	helpFilter  = "/ filter"
	helpToggle  = "tab always use"
	toggleLabel = "Always use this answer in this repo"
)

type Option struct {
	Label     string
	Value     string
	Badge     string
	Separator bool
}

type SelectList struct {
	options   []Option
	shown     []int
	cursor    int
	offset    int
	query     string
	filtering bool
	toggle    bool
	remember  bool
}

type SelectParams struct {
	Options  []Option
	Start    string
	Toggle   bool
	ToggleOn bool
}

func NewSelectList(params SelectParams) *SelectList {
	l := &SelectList{options: params.Options, toggle: params.Toggle, remember: params.Toggle && params.ToggleOn}
	l.refilter()
	l.snap()
	l.StartOn(params.Start)
	return l
}

func (l *SelectList) StartOn(value string) {
	for i, at := range l.shown {
		if value != "" && l.options[at].Value == value && !l.options[at].Separator {
			l.cursor = i
			return
		}
	}
}

func (l *SelectList) Value() string {
	if l.cursor < 0 || l.cursor >= len(l.shown) {
		return ""
	}
	return l.options[l.shown[l.cursor]].Value
}

func (l *SelectList) Filtering() bool {
	return l.filtering
}

func (l *SelectList) Remember() bool {
	return l.remember
}

func (l *SelectList) Update(msg tea.Msg) (Verdict, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return Stay, nil
	}
	if l.filtering {
		return l.filter(key), nil
	}
	switch key.String() {
	case "up", "k":
		l.move(-1)
	case "down", "j":
		l.move(1)
	case "enter":
		if len(l.shown) > 0 {
			return Done, nil
		}
	case "esc":
		if l.query == "" {
			return Back, nil
		}
		l.query = ""
		l.refilter()
		l.snap()
	case "/":
		l.filtering = true
	case "tab":
		l.remember = l.toggle && !l.remember
	}
	return Stay, nil
}

func (l *SelectList) filter(key tea.KeyMsg) Verdict {
	switch typed := key.String(); typed {
	case "esc":
		l.filtering = false
	case "backspace":
		l.query = l.query[:max(len(l.query)-1, 0)]
		l.filtering = l.query != ""
		l.refilter()
		l.snap()
	case "enter":
		if len(l.shown) > 0 {
			l.filtering = false
			return Done
		}
	case "up":
		l.move(-1)
	case "down":
		l.move(1)
	default:
		if len(typed) == 1 && typed >= " " {
			l.query += typed
			l.refilter()
			l.snap()
		}
	}
	return Stay
}

func (l *SelectList) refilter() {
	l.shown = l.shown[:0]
	query := strings.ToLower(l.query)
	for i, o := range l.options {
		if query != "" && (o.Separator || !strings.Contains(strings.ToLower(o.Label), query)) {
			continue
		}
		l.shown = append(l.shown, i)
	}
	l.cursor = min(l.cursor, max(len(l.shown)-1, 0))
}

func (l *SelectList) selectable(i int) bool {
	return i >= 0 && i < len(l.shown) && !l.options[l.shown[i]].Separator
}

func (l *SelectList) snap() {
	if !l.selectable(l.cursor) {
		l.move(1)
	}
	if !l.selectable(l.cursor) {
		l.move(-1)
	}
}

func (l *SelectList) move(step int) {
	for i := l.cursor + step; i >= 0 && i < len(l.shown); i += step {
		if l.selectable(i) {
			l.cursor = i
			return
		}
	}
}

func (l *SelectList) View(size Size) string {
	if len(l.options) == 0 {
		return ""
	}
	var b strings.Builder
	if l.filtering || l.query != "" {
		b.WriteString(output.Muted.Render("/ " + l.query))
		if l.filtering {
			b.WriteString(reverse.Render(" "))
		}
		b.WriteString("\n\n")
	}
	rows := len(l.shown)
	if size.Height > 0 {
		rows = max(1, size.Height-l.overhead())
	}
	l.offset = min(max(l.offset, l.cursor-rows+1), l.cursor)
	end := min(l.offset+rows, len(l.shown))
	for i := l.offset; i < end; i++ {
		b.WriteString(l.row(l.options[l.shown[i]], i == l.cursor, size.Width))
		if i < end-1 {
			b.WriteString("\n")
		}
	}
	if len(l.shown) == 0 {
		b.WriteString(output.Muted.Render(output.Indent + noMatches))
	}
	if l.toggle {
		check := checkOff
		if l.remember {
			check = checkOn
		}
		b.WriteString("\n\n" + strings.Repeat(" ", rowPrefix+rowLead) + check + " " + output.Muted.Render(toggleLabel))
	}
	return b.String()
}

func (l *SelectList) overhead() int {
	lines := 0
	if l.filtering || l.query != "" {
		lines += 2
	}
	if l.toggle {
		lines += 2
	}
	return lines
}

func (l *SelectList) row(o Option, selected bool, width int) string {
	if o.Separator {
		return output.Muted.Render(output.Indent + strings.Repeat("─", max(width-4, 4)))
	}
	fill := max(width-rowPrefix-rowLead-lipgloss.Width(o.Label)-lipgloss.Width(o.Badge)-rowTagGap, 1)
	if selected {
		block := " " + o.Label + strings.Repeat(" ", fill) + o.Badge + strings.Repeat(" ", rowTagGap)
		return Accent.Render("▌›") + tinted.Render(block)
	}
	return strings.Repeat(" ", rowPrefix+rowLead) + o.Label + strings.Repeat(" ", fill) + output.Muted.Render(o.Badge) + strings.Repeat(" ", rowTagGap)
}

func (l *SelectList) Help() Help {
	if l.filtering {
		return Help{Modal: filterHelp}
	}
	if l.toggle {
		return Help{Actions: []string{helpToggle, helpFilter}}
	}
	return Help{Actions: []string{helpFilter}}
}
