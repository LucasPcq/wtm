package form

import (
	"strings"

	"github.com/LucasPcq/wtm/next/internal/ui/output"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type TextList struct {
	input   textinput.Model
	entries []string
	check   func(check EntryCheck) error
	badge   func(entry string) string
	err     error
}

type EntryCheck struct {
	Entries []string
	From    int
}

type TextListParams struct {
	Entries []string
	Check   func(check EntryCheck) error
	Badge   func(entry string) string
}

func NewTextList(params TextListParams) *TextList {
	input := textinput.New()
	input.Prompt = Accent.Render("› ")
	input.Focus()
	return &TextList{input: input, entries: append([]string(nil), params.Entries...), check: params.Check, badge: params.Badge}
}

func (l *TextList) Entries() []string {
	return l.entries
}

func (l *TextList) Update(msg tea.Msg) (Verdict, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		var cmd tea.Cmd
		l.input, cmd = l.input.Update(msg)
		return Stay, cmd
	}
	switch key.String() {
	case "tab":
		l.add()
		return Stay, nil
	case "enter":
		return l.confirm(), nil
	case "esc":
		return Back, nil
	case "backspace":
		if l.input.Value() == "" && len(l.entries) > 0 {
			l.entries, l.err = l.entries[:len(l.entries)-1], nil
			return Stay, nil
		}
	}
	var cmd tea.Cmd
	l.input, cmd = l.input.Update(msg)
	l.err = nil
	return Stay, cmd
}

func (l *TextList) add() bool {
	typed := strings.TrimSpace(l.input.Value())
	if typed == "" {
		return true
	}
	entries := append(append([]string(nil), l.entries...), typed)
	if l.err = l.check(EntryCheck{Entries: entries, From: len(l.entries)}); l.err != nil {
		return false
	}
	l.entries = entries
	l.input.SetValue("")
	return true
}

func (l *TextList) confirm() Verdict {
	if !l.add() {
		return Stay
	}
	if l.err = l.check(EntryCheck{Entries: l.entries}); l.err != nil {
		return Stay
	}
	return Done
}

func (l *TextList) View(size Size) string {
	l.input.Width = max(10, size.Width-4)
	var b strings.Builder
	b.WriteString(l.input.View())
	for _, entry := range l.entries {
		b.WriteString("\n" + output.Indent + entry)
		if badge := l.badge(entry); badge != "" {
			b.WriteString("  " + output.Muted.Render(badge))
		}
	}
	if l.err != nil {
		b.WriteString("\n\n" + errorBanner(l.err.Error(), size.Width))
	}
	return b.String()
}

func (l *TextList) Help() Help {
	return Help{Actions: []string{"tab add another", "backspace remove last"}, Rowless: true}
}
