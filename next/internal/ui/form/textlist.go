package form

import (
	"strings"

	"github.com/LucasPcq/wtm/next/internal/ui/output"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type TextList struct {
	input    textinput.Model
	entries  []string
	check    func(check EntryCheck) error
	badge    func(entry string) string
	err      error
	checking bool
	queued   []tea.KeyMsg
}

type EntryCheck struct {
	Entries []string
	From    int
}

type entryChecked struct {
	check   EntryCheck
	adding  bool
	confirm bool
	err     error
}

func NewTextList(entries []string, check func(check EntryCheck) error, badge func(entry string) string) *TextList {
	input := textinput.New()
	input.Focus()
	return &TextList{input: input, entries: append([]string(nil), entries...), check: check, badge: badge}
}

func (l *TextList) Entries() []string {
	return l.entries
}

func (l *TextList) Update(msg tea.Msg) (Verdict, tea.Cmd) {
	if checked, ok := msg.(entryChecked); ok {
		return l.checked(checked)
	}
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		var cmd tea.Cmd
		l.input, cmd = l.input.Update(msg)
		return Stay, cmd
	}
	if l.checking {
		l.queued = append(l.queued, key)
		return Stay, nil
	}
	switch key.String() {
	case "tab":
		return Stay, l.add(false)
	case "enter":
		return Stay, l.add(true)
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

func (l *TextList) add(confirm bool) tea.Cmd {
	typed := strings.TrimSpace(l.input.Value())
	if typed == "" && confirm {
		return l.run(entryChecked{check: EntryCheck{Entries: l.entries}, confirm: true})
	}
	if typed == "" {
		return nil
	}
	entries := append(append([]string(nil), l.entries...), typed)
	return l.run(entryChecked{check: EntryCheck{Entries: entries, From: len(l.entries)}, adding: true, confirm: confirm})
}

func (l *TextList) run(pending entryChecked) tea.Cmd {
	l.checking = true
	check := l.check
	return func() tea.Msg {
		pending.err = check(pending.check)
		return pending
	}
}

func (l *TextList) checked(done entryChecked) (Verdict, tea.Cmd) {
	l.checking, l.err = false, done.err
	switch {
	case done.err != nil:
	case done.adding && done.confirm:
		l.accept(done.check.Entries)
		return Stay, l.run(entryChecked{check: EntryCheck{Entries: l.entries}, confirm: true})
	case done.adding:
		l.accept(done.check.Entries)
	default:
		l.queued = nil
		return Done, nil
	}
	return l.replay()
}

func (l *TextList) accept(entries []string) {
	l.entries = entries
	l.input.SetValue("")
}

func (l *TextList) replay() (Verdict, tea.Cmd) {
	var cmds []tea.Cmd
	for len(l.queued) > 0 && !l.checking {
		key := l.queued[0]
		l.queued = l.queued[1:]
		verdict, cmd := l.Update(key)
		cmds = append(cmds, cmd)
		if verdict != Stay {
			l.queued = nil
			return verdict, tea.Batch(cmds...)
		}
	}
	return Stay, tea.Batch(cmds...)
}

func (l *TextList) View(size Size, st *Styles) string {
	l.input.Prompt = st.accent.Render("› ")
	l.input.Width = max(10, size.Width-4)
	var b strings.Builder
	b.WriteString(l.input.View())
	for _, entry := range l.entries {
		b.WriteString("\n" + output.Indent + entry)
		if badge := l.badge(entry); badge != "" {
			b.WriteString("  " + st.Muted.Render(badge))
		}
	}
	if l.err != nil {
		b.WriteString("\n\n" + errorBanner(l.err.Error(), size.Width, st))
	}
	return b.String()
}

func (l *TextList) Help() Help {
	return Help{Actions: []string{"tab add another", "backspace remove last"}, Rowless: true}
}
