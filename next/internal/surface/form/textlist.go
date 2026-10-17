package form

import (
	"strings"

	"github.com/LucasPcq/wtm/next/internal/surface/output"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type textList struct {
	input   textinput.Model
	entries []string
	check   func(check entryCheck) error
	badge   func(entry string) string
	err     error
}

type entryCheck struct {
	Entries []string
	From    int
}

type textListParams struct {
	Entries []string
	Check   func(check entryCheck) error
	Badge   func(entry string) string
}

func newTextList(params textListParams) *textList {
	input := textinput.New()
	input.Prompt = accent.Render("› ")
	input.Focus()
	return &textList{input: input, entries: append([]string(nil), params.Entries...), check: params.Check, badge: params.Badge}
}

func (l *textList) update(msg tea.Msg) (verdict, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		var cmd tea.Cmd
		l.input, cmd = l.input.Update(msg)
		return stay, cmd
	}
	switch key.String() {
	case "tab":
		l.add()
		return stay, nil
	case "enter":
		return l.confirm(), nil
	case "esc":
		return back, nil
	case "backspace":
		if l.input.Value() == "" && len(l.entries) > 0 {
			l.entries, l.err = l.entries[:len(l.entries)-1], nil
			return stay, nil
		}
	}
	var cmd tea.Cmd
	l.input, cmd = l.input.Update(msg)
	l.err = nil
	return stay, cmd
}

func (l *textList) add() bool {
	typed := strings.TrimSpace(l.input.Value())
	if typed == "" {
		return true
	}
	entries := append(append([]string(nil), l.entries...), typed)
	if l.err = l.check(entryCheck{Entries: entries, From: len(l.entries)}); l.err != nil {
		return false
	}
	l.entries = entries
	l.input.SetValue("")
	return true
}

func (l *textList) confirm() verdict {
	if !l.add() {
		return stay
	}
	if l.err = l.check(entryCheck{Entries: l.entries}); l.err != nil {
		return stay
	}
	return done
}

func (l *textList) view(size size) string {
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

func (l *textList) help() help {
	return help{Actions: []string{"tab add another", "backspace remove last"}, Rowless: true}
}
