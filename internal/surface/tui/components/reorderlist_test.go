package components

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func newTestReorder() ReorderListModel {
	return NewReorderList(NewReorderListParams{
		Title: "Order",
		Items: []ReorderItem{
			{Label: "build (task)", Value: "build"},
			{Label: "db (service)", Value: "db"},
			{Label: "server (service)", Value: "server"},
		},
	})
}

func key(t tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: t} }

func runeKey(r rune) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}} }

func send(m ReorderListModel, msgs ...tea.Msg) ReorderListModel {
	for _, msg := range msgs {
		m, _ = m.Update(msg)
	}
	return m
}

func TestReorderInitialOrder(t *testing.T) {
	m := newTestReorder()
	got := strings.Join(m.Values(), ",")
	if got != "build,db,server" {
		t.Fatalf("expected build,db,server, got %s", got)
	}
}

func TestReorderMoveDown(t *testing.T) {
	// Cursor on first item, move it down once: build swaps past db.
	m := send(newTestReorder(), key(tea.KeyShiftDown))
	if got := strings.Join(m.Values(), ","); got != "db,build,server" {
		t.Fatalf("expected db,build,server, got %s", got)
	}
	// Cursor should follow the moved item (now at index 1).
	if m.cursor != 1 {
		t.Fatalf("expected cursor 1, got %d", m.cursor)
	}
}

func TestReorderMoveUp(t *testing.T) {
	// Navigate to last item, then move it up to the top.
	m := send(newTestReorder(),
		runeKey('j'), runeKey('j'), // cursor -> server (index 2)
		key(tea.KeyShiftUp), key(tea.KeyShiftUp), // server -> index 0
	)
	if got := strings.Join(m.Values(), ","); got != "server,build,db" {
		t.Fatalf("expected server,build,db, got %s", got)
	}
	if m.cursor != 0 {
		t.Fatalf("expected cursor 0, got %d", m.cursor)
	}
}

func TestReorderMoveBoundaries(t *testing.T) {
	// Moving up at the top is a no-op.
	m := send(newTestReorder(), key(tea.KeyShiftUp))
	if got := strings.Join(m.Values(), ","); got != "build,db,server" {
		t.Fatalf("top move-up should be no-op, got %s", got)
	}
	// Moving down past the bottom is a no-op.
	m = send(newTestReorder(),
		runeKey('j'), runeKey('j'), // cursor at last item
		key(tea.KeyShiftDown),
	)
	if got := strings.Join(m.Values(), ","); got != "build,db,server" {
		t.Fatalf("bottom move-down should be no-op, got %s", got)
	}
}

func TestReorderUppercaseKeysMove(t *testing.T) {
	// 'J' moves down, 'K' moves up — for terminals without shift+arrow.
	m := send(newTestReorder(), runeKey('J'))
	if got := strings.Join(m.Values(), ","); got != "db,build,server" {
		t.Fatalf("J should move down, got %s", got)
	}
	m = send(m, runeKey('K'))
	if got := strings.Join(m.Values(), ","); got != "build,db,server" {
		t.Fatalf("K should move up, got %s", got)
	}
}

func TestReorderDoneAndAborted(t *testing.T) {
	m := send(newTestReorder(), key(tea.KeyEnter))
	if !m.Done() || m.Aborted() {
		t.Fatalf("enter should set done, got done=%v aborted=%v", m.Done(), m.Aborted())
	}
	m = send(newTestReorder(), key(tea.KeyEsc))
	if m.Done() || !m.Aborted() {
		t.Fatalf("esc should set aborted, got done=%v aborted=%v", m.Done(), m.Aborted())
	}
}
