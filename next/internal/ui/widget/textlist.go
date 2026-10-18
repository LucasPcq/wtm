package widget

import (
	"strings"

	"github.com/LucasPcq/wtm/next/internal/ui/output"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// A TextList is a [Widget] that collects a list of typed entries: tab adds the entry typed, enter adds it and confirms the list, backspace on an empty input removes the last one.
// Each change is checked off the event loop before it is accepted, and keys typed meanwhile wait for the check.
type TextList struct {
	input    textinput.Model
	entries  []string
	check    func(check EntryCheck) error
	badge    func(entry string) string
	err      error
	checking bool
	queued   []tea.KeyMsg
}

// An EntryCheck is the list a [TextList] asks its check to accept: every entry, and the index of the first one it added.
type EntryCheck struct {
	Entries []string
	From    int
}

// An entryChecked brings a check's verdict back to the event loop, with what to do once it is accepted.
type entryChecked struct {
	check   EntryCheck
	adding  bool
	confirm bool
	err     error
}

// NewTextList returns a text list holding a copy of entries; check accepts a change or returns the error to show, and badge returns the note drawn beside an entry.
func NewTextList(entries []string, check func(check EntryCheck) error, badge func(entry string) string) *TextList {
	input := textinput.New()
	input.Focus()
	return &TextList{input: input, entries: append([]string(nil), entries...), check: check, badge: badge}
}

// Entries returns the entries accepted.
func (l *TextList) Entries() []string {
	return l.entries
}

// Update handles a check that landed, a key, or a message of the text input, such as its cursor's blink.
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

// add checks the list with the entry typed added, and confirms it once accepted when confirm is set; with nothing typed, it checks the list as it is to confirm it.
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

// run checks pending off the event loop, keys waiting until its verdict lands.
func (l *TextList) run(pending entryChecked) tea.Cmd {
	l.checking = true
	check := l.check
	return func() tea.Msg {
		pending.err = check(pending.check)
		return pending
	}
}

// checked takes a check's verdict: a refused change shows its error, an accepted entry joins the list (and an enter checks the whole list next), an accepted list answers the field; the keys that waited are replayed after.
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

// replay handles the keys that waited for a check, in order, until one starts another check or ends the field.
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

// View draws the input, then each entry with its badge, then the error of a refused change.
func (l *TextList) View(size Size, st *Styles) string {
	l.input.Prompt = st.Accent.Render("› ")
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

// Help returns the keys that add and remove entries; the list has no rows to navigate.
func (l *TextList) Help() Help {
	return Help{Actions: []string{"tab add another", "backspace remove last"}, Rowless: true}
}
