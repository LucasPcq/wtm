package widget

import (
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	hookAdd          = "+ Add a command"
	hookDone         = "✓ Done"
	hookCommandLabel = "Command"
	hookFolderLabel  = "Folder "
	hookGoOn         = "Go on if it fails"
	hookRoot         = "the worktree's root"
	hookEditHelp     = "tab next field • space toggle • enter save • esc cancel"
)

var errNoCommand = errors.New("type a command, or esc to cancel")

// A HookList is a [Widget] that edits a list of hooks in the order they run: enter edits the row under the cursor (its command, the folder it runs in, whether a failure lets the next ones run), adds one on "Add a command" or confirms the list on "Done"; d removes a hook, shift+↑↓ moves it. The list is checked off the event loop before it is confirmed.
type HookList struct {
	hooks    []project.Hook
	cursor   int
	check    func(hooks []project.Hook) error
	err      error
	checking bool
	edit     *hookEdit
}

// hookEdit is the hook being edited: its row (len(hooks) for a new one), its two inputs, its switch and the one focused (0 command, 1 folder, 2 the switch).
type hookEdit struct {
	at      int
	command textinput.Model
	folder  textinput.Model
	goOn    bool
	focus   int
	err     error
}

// hooksChecked brings a check's verdict back to the event loop.
type hooksChecked struct{ err error }

// NewHookList returns a hook list holding a copy of hooks; check accepts the list or returns the error to show.
func NewHookList(hooks []project.Hook, check func(hooks []project.Hook) error) *HookList {
	return &HookList{hooks: slices.Clone(hooks), check: check}
}

// Hooks returns the hooks as edited, in order.
func (l *HookList) Hooks() []project.Hook {
	return l.hooks
}

// Update handles a check that landed, then a key: the edit's while a hook is edited, the list's otherwise.
func (l *HookList) Update(msg tea.Msg) (Verdict, tea.Cmd) {
	if checked, ok := msg.(hooksChecked); ok {
		l.checking, l.err = false, checked.err
		if checked.err != nil {
			return Stay, nil
		}
		return Done, nil
	}
	if l.edit != nil {
		return Stay, l.editKey(msg)
	}

	key, ok := msg.(tea.KeyMsg)
	if !ok || l.checking {
		return Stay, nil
	}
	return l.listKey(key)
}

// listKey moves the cursor, removes or moves the hook under it, or opens a row.
func (l *HookList) listKey(key tea.KeyMsg) (Verdict, tea.Cmd) {
	rows := len(l.hooks) + 2
	l.err = nil
	switch key.String() {
	case "up", "k":
		l.cursor = (l.cursor + rows - 1) % rows
	case "down", "j":
		l.cursor = (l.cursor + 1) % rows
	case "shift+up":
		if l.cursor > 0 && l.cursor < len(l.hooks) {
			l.hooks[l.cursor-1], l.hooks[l.cursor] = l.hooks[l.cursor], l.hooks[l.cursor-1]
			l.cursor--
		}
	case "shift+down":
		if l.cursor < len(l.hooks)-1 {
			l.hooks[l.cursor+1], l.hooks[l.cursor] = l.hooks[l.cursor], l.hooks[l.cursor+1]
			l.cursor++
		}
	case "d", "delete":
		if l.cursor < len(l.hooks) {
			l.hooks = slices.Delete(l.hooks, l.cursor, l.cursor+1)
		}
	case "esc":
		return Back, nil
	case "enter":
		return l.open()
	}

	return Stay, nil
}

// open edits the hook under the cursor, a new one on "Add a command", or checks the list on "Done".
func (l *HookList) open() (Verdict, tea.Cmd) {
	if l.cursor == len(l.hooks)+1 {
		l.checking = true
		check, hooks := l.check, slices.Clone(l.hooks)
		return Stay, func() tea.Msg { return hooksChecked{err: check(hooks)} }
	}

	var hook project.Hook
	if l.cursor < len(l.hooks) {
		hook = l.hooks[l.cursor]
	}
	command, folder := textinput.New(), textinput.New()
	command.SetValue(hook.Cmd)
	folder.SetValue(hook.Cwd)
	folder.Placeholder = hookRoot
	command.Focus()
	l.edit = &hookEdit{at: l.cursor, command: command, folder: folder, goOn: hook.ContinueOnError}
	return Stay, textinput.Blink
}

// editKey moves between the edit's fields, flips its switch, saves it into the list or leaves it as it was.
func (l *HookList) editKey(msg tea.Msg) tea.Cmd {
	edit := l.edit
	key, ok := msg.(tea.KeyMsg)
	switch {
	case ok && key.String() == "esc":
		l.edit = nil
		return nil
	case ok && key.String() == "enter":
		return l.save()
	case ok && (key.String() == "tab" || key.String() == "shift+tab"):
		step := 1
		if key.String() == "shift+tab" {
			step = 2
		}
		edit.focus = (edit.focus + step) % 3
		edit.command.Blur()
		edit.folder.Blur()
		if edit.focus == 0 {
			edit.command.Focus()
		}
		if edit.focus == 1 {
			edit.folder.Focus()
		}
		return nil
	case ok && edit.focus == 2:
		if key.String() == " " || key.String() == "space" {
			edit.goOn = !edit.goOn
		}
		return nil
	}

	var cmd tea.Cmd
	edit.err = nil
	if edit.focus == 0 {
		edit.command, cmd = edit.command.Update(msg)
	} else {
		edit.folder, cmd = edit.folder.Update(msg)
	}
	return cmd
}

// save puts the hook edited in its row, refusing one with no command.
func (l *HookList) save() tea.Cmd {
	edit := l.edit
	hook := project.Hook{Cmd: strings.TrimSpace(edit.command.Value()), Cwd: strings.TrimSpace(edit.folder.Value()), ContinueOnError: edit.goOn}
	if hook.Cmd == "" {
		edit.err = errNoCommand
		return nil
	}

	if edit.at < len(l.hooks) {
		l.hooks[edit.at] = hook
	} else {
		l.hooks = append(l.hooks, hook)
		l.cursor = len(l.hooks) - 1
	}
	l.edit = nil
	return nil
}

// View draws the list, each hook with the folder it runs in and whether it goes on after a failure, then its two actions; or the hook being edited.
func (l *HookList) View(size Size, st *Styles) string {
	if l.edit != nil {
		return l.editView(size, st)
	}

	rows := make([]string, 0, len(l.hooks)+2)
	for i, hook := range l.hooks {
		row := strconv.Itoa(i+1) + ". " + hook.Cmd
		if hook.Cwd != "" {
			row += st.Muted.Render(" · in " + hook.Cwd)
		}
		if hook.ContinueOnError {
			row += st.Muted.Render(" · goes on if it fails")
		}
		rows = append(rows, row)
	}
	rows = append(rows, hookAdd, hookDone)

	var b strings.Builder
	for i, row := range rows {
		if i > 0 {
			b.WriteString("\n")
		}
		if i == l.cursor {
			b.WriteString(st.Accent.Render("› ") + row)
			continue
		}
		b.WriteString("  " + row)
	}
	if l.err != nil {
		b.WriteString("\n\n" + errorBanner(l.err, size.Width, st))
	}

	return b.String()
}

func (l *HookList) editView(size Size, st *Styles) string {
	edit := l.edit
	edit.command.Prompt = st.Accent.Render("› ")
	edit.folder.Prompt = st.Accent.Render("› ")
	edit.command.Width = max(10, size.Width-14)
	edit.folder.Width = max(10, size.Width-14)
	box := "[ ]"
	if edit.goOn {
		box = "[x]"
	}
	marker := func(focus int) string {
		if edit.focus == focus {
			return st.Accent.Render("› ")
		}
		return "  "
	}

	view := marker(0) + hookCommandLabel + " " + edit.command.View() + "\n" +
		marker(1) + hookFolderLabel + " " + edit.folder.View() + "\n" +
		marker(2) + box + " " + hookGoOn
	if edit.err != nil {
		view += "\n\n" + errorBanner(edit.err, size.Width, st)
	}
	return view
}

// Help returns the keys of the list, or the edit's own help line while a hook is edited.
func (l *HookList) Help() Help {
	if l.edit != nil {
		return Help{Modal: "  " + hookEditHelp}
	}
	return Help{Actions: []string{"d remove", "shift+↑↓ move", "enter edit"}}
}
