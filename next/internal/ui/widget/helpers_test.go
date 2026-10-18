package widget

import (
	"io"
	"strings"
	"time"

	"github.com/LucasPcq/wtm/next/internal/ui/output"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

const settleWithin = 100 * time.Millisecond

// keyOf is the key press name stands for, as formtest.Key, which these tests cannot import: formtest imports form, which imports widget.
func keyOf(name string) tea.KeyMsg {
	switch name {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "space":
		return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(name)}
}

func plain(view string) string {
	lines := strings.Split(ansi.Strip(view), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

// settled runs the checks cmd starts on l, and those their verdicts start in turn, and returns the verdict they end on.
func settled(l *TextList, cmd tea.Cmd) Verdict {
	verdict := Stay
	for _, msg := range messages(cmd) {
		if checked, ok := msg.(entryChecked); ok {
			var next tea.Cmd
			verdict, next = l.Update(checked)
			if verdict == Stay {
				verdict = settled(l, next)
			}
		}
	}
	return verdict
}

// textSettled runs the check cmd starts on x and returns the verdict it ends on.
func textSettled(x *Text, cmd tea.Cmd) Verdict {
	verdict := Stay
	for _, msg := range messages(cmd) {
		if checked, ok := msg.(textChecked); ok {
			verdict, _ = x.Update(checked)
		}
	}
	return verdict
}

// messages returns what cmd returns, a batch flattened; nil when it does not return within settleWithin.
func messages(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	result := make(chan tea.Msg, 1)
	go func() { result <- cmd() }()
	select {
	case msg := <-result:
		batch, ok := msg.(tea.BatchMsg)
		if !ok {
			return []tea.Msg{msg}
		}
		var all []tea.Msg
		for _, inner := range batch {
			all = append(all, messages(inner)...)
		}
		return all
	case <-time.After(settleWithin):
		return nil
	}
}

func plainStyles() *Styles {
	return NewStyles(output.StylesOf(io.Discard))
}

func pressed(m *MultiSelect, keys ...string) Verdict {
	verdict := Stay
	for _, key := range keys {
		verdict, _ = m.Update(keyOf(key))
	}
	return verdict
}
