package runview

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.focused {
		return m.handleFocusKey(msg)
	}
	if m.filtering {
		return m.handleFilterKey(msg)
	}

	// A refusal answers one keystroke; the next one is a fresh question.
	m.notice = ""

	// Any key the reader presses is them taking the cursor back from the run.
	m.following = false

	switch msg.String() {
	case domain.RunViewFocusKey:
		return m.focus()
	case keyEscape:
		return m.dismiss()
	case keyQuit, keyInterrupt:
		return m.detach()
	case keyUp, keyVimUp:
		return m.move(-1)
	case keyDown, keyVimDown:
		return m.move(1)
	case keyFilter:
		m.filtering = true
		return m, nil
	case keyRefresh:
		return m, m.refreshCmd()
	case keyPageUp:
		return m.scroll(m.layout().PaneRows), nil
	case keyPageDown:
		return m.scroll(-m.layout().PaneRows), nil
	case keyScrollUp:
		return m.scroll(domain.RunViewScrollLines), nil
	case keyScrollDwn:
		return m.scroll(-domain.RunViewScrollLines), nil
	case keyLive:
		return m.scrollToLive(), nil
	case keyOpenURL:
		return m, m.openSelectedURL()
	}
	return m, nil
}

// handleMouse routes the wheel to whatever it is over: the pane scrolls its own
// scrollback, the list moves the selection. A job that has the keyboard keeps
// it — the wheel must not move a cursor the reader handed to the process.
func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.focused || m.filtering {
		return m, nil
	}

	up, ok := wheelDirection(msg)
	if !ok {
		return m, nil
	}

	// The wheel is the reader acting, exactly as a keystroke is: it dismisses a
	// standing refusal and takes the cursor back from the run, which would
	// otherwise pull the selection away on the next event.
	m.notice = ""
	m.following = false

	if m.overSidebar(msg) {
		if up {
			return m.move(-1)
		}
		return m.move(1)
	}

	if up {
		return m.scroll(domain.RunViewScrollLines), nil
	}
	return m.scroll(-domain.RunViewScrollLines), nil
}

func wheelDirection(msg tea.MouseMsg) (up, isWheel bool) {
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		return true, true
	case tea.MouseButtonWheelDown:
		return false, true
	}
	return false, false
}

// overSidebar answers where the pointer is. A frame too narrow to draw the list
// has none, and every wheel event is then the pane's.
func (m Model) overSidebar(msg tea.MouseMsg) bool {
	layout := m.layout()
	if !layout.SidebarVisible {
		return false
	}
	return msg.X >= layout.Sidebar.X && msg.X < layout.Sidebar.X+layout.Sidebar.Width
}

// handleFocusKey hands the keyboard to the job: every key it can encode is
// written to the job's stdin, Ctrl+C included — interrupting the child is the
// whole point of focus. Even the exit key goes through, since a lone press
// belongs to the job; only a second one right behind it means the reader.
func (m Model) handleFocusKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == domain.RunViewFocusExitKey {
		if rules.ExitsFocus(rules.FocusExitParams{Previous: m.lastExitKey, Now: time.Now()}) {
			m.focused, m.lastExitKey = false, time.Time{}
			return m, nil
		}
		m.lastExitKey = time.Now()
	} else {
		// Anything in between makes the two presses two keystrokes for the job.
		m.lastExitKey = time.Time{}
	}

	stream := m.panes.stream(m.selected)
	if stream == nil {
		m.focused = false
		return m, nil
	}
	encoded := rules.EncodeKeyStroke(strokeOf(msg))
	if len(encoded) == 0 {
		return m, nil
	}
	return m, writeCmd(writeParams{Key: m.selected, Stream: stream, Bytes: encoded})
}

// focus is refused for a pane the job is not behind: a log file has no stdin,
// and a keystroke silently going nowhere reads as a frozen terminal.
func (m Model) focus() (tea.Model, tea.Cmd) {
	if m.panes.stream(m.selected) == nil {
		m.notice = fmt.Sprintf(domain.RunViewNotAttachableFmt, m.selected.job())
		return m, nil
	}
	m.focused, m.notice, m.lastExitKey = true, "", time.Time{}
	return m.scrollToLive(), nil
}

// openSelectedURL does nothing for a job that publishes none: a key pressed on
// a job with nothing to open is not a mistake to report. A browser that refused
// to open is one — the reader would otherwise wait for a window that is not
// coming.
func (m Model) openSelectedURL() tea.Cmd {
	url := m.sequence.urls[m.selected]
	if url == "" || m.open == nil {
		return nil
	}
	open := m.open
	return func() tea.Msg {
		if err := open(url); err != nil {
			return openFailedMsg{err: err}
		}
		return nil
	}
}

func strokeOf(msg tea.KeyMsg) domain.KeyStroke {
	if msg.Type == tea.KeyRunes {
		return domain.KeyStroke{Runes: msg.Runes, Alt: msg.Alt}
	}
	return domain.KeyStroke{Name: msg.Type.String(), Alt: msg.Alt}
}

// handleFilterKey reads the filter box. Every keystroke re-resolves the
// selection: a filter that hides the selected job moves the cursor rather than
// leaving a pane on screen the list no longer offers.
func (m Model) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case keyInterrupt:
		return m.detach()
	case keyEscape:
		m.filtering, m.filter = false, ""
		return m.setSelection(m.resolveSelection())
	case keyEnter:
		m.filtering = false
		return m, nil
	case keyBackspace:
		if m.filter == "" {
			return m, nil
		}
		runes := []rune(m.filter)
		m.filter = string(runes[:len(runes)-1])
		return m.setSelection(m.resolveSelection())
	}

	if msg.Type != tea.KeyRunes && msg.Type != tea.KeySpace {
		return m, nil
	}
	m.filter += string(msg.Runes)
	return m.setSelection(m.resolveSelection())
}

// detach leaves the view. Nothing is stopped: cancelling ends the reporting of
// a run in progress, not the run, and the jobs behind the dropped streams keep
// running.
func (m Model) detach() (tea.Model, tea.Cmd) {
	m.cancel()
	m.panes.closeAll()
	return m, tea.Quit
}

// dismiss takes the abort report off the frame and gives its rows back to the
// panes under it. With no report on screen the key does nothing: pressing esc
// before the run fails would otherwise silence a report that is not written
// yet, and the reader would never learn why the profile stopped.
func (m Model) dismiss() (tea.Model, tea.Cmd) {
	if len(m.report()) == 0 {
		return m, nil
	}
	m.dismissed = true
	model, cmd := m.applySize()
	return model, cmd
}

func (m Model) move(delta int) (tea.Model, tea.Cmd) {
	visible := m.visible()
	if len(visible) == 0 {
		return m, nil
	}
	index := rules.ClampIndex(m.selectedIndex()+delta, len(visible))
	return m.setSelection(viewKey(visible[index]))
}

// setSelection moves the cursor and releases the pane it leaves behind: only
// the selected job holds a subscription, so the one being left has to give its
// own up before the next is opened.
func (m Model) setSelection(key jobKey) (Model, tea.Cmd) {
	if key != m.selected {
		if !m.sequenceHolds(m.selected) {
			m.panes.release(m.selected)
		}
		m.selected, m.focused = key, false
	}
	m.panes.follow(key)
	// The offset is measured over rows rather than over jobs: a worktree heading
	// takes a row of the sidebar like any other, and counting jobs alone would
	// scroll the last one out of the panel it was measured to fit.
	rows := m.rows()
	m.offset = rules.DashboardScrollOffset(rules.DashboardScrollParams{
		Cursor:  selectedRowIndex(rows, m.selected),
		Total:   len(rows),
		Visible: m.listViewport(),
		Offset:  m.offset,
	})
	return m.fillSelectedPane()
}

func (m Model) scroll(lines int) Model {
	entry, held := m.panes.entry(m.selected)
	if !held {
		return m
	}
	entry.pane.ScrollUp(lines)
	return m
}

func (m Model) scrollToLive() Model {
	entry, held := m.panes.entry(m.selected)
	if !held {
		return m
	}
	entry.pane.ScrollToLive()
	return m
}
