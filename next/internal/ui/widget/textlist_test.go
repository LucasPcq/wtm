package widget

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnEntryIsCheckedOffTheLoop(t *testing.T) {
	var checked []EntryCheck
	l := NewTextList(nil, func(c EntryCheck) error { checked = append(checked, c); return nil }, func(string) string { return "" })
	l.input.SetValue("feat/a")

	_, cmd := l.Update(keyOf("tab"))
	require.NotNil(t, cmd)
	assert.Empty(t, checked)
	assert.Empty(t, l.Entries())

	settled(l, cmd)

	assert.Equal(t, []EntryCheck{{Entries: []string{"feat/a"}}}, checked)
	assert.Equal(t, []string{"feat/a"}, l.Entries())
	assert.Empty(t, l.input.Value())
}

func TestEnterChecksTheTypedEntryThenTheWholeList(t *testing.T) {
	var checked []EntryCheck
	l := NewTextList([]string{"feat/a"}, func(c EntryCheck) error { checked = append(checked, c); return nil }, func(string) string { return "" })
	l.input.SetValue("feat/b")

	_, cmd := l.Update(keyOf("enter"))

	assert.Equal(t, Done, settled(l, cmd))
	assert.Equal(t, []EntryCheck{{Entries: []string{"feat/a", "feat/b"}, From: 1}, {Entries: []string{"feat/a", "feat/b"}}}, checked)
}

func TestKeysTypedWhileAnEntryIsCheckedAreKeptInOrder(t *testing.T) {
	l := NewTextList(nil, func(EntryCheck) error { return nil }, func(string) string { return "" })
	l.input.SetValue("feat/a")

	_, cmd := l.Update(keyOf("tab"))
	l.Update(keyOf("b"))
	l.Update(keyOf("enter"))

	assert.Equal(t, Done, settled(l, cmd))
	assert.Equal(t, []string{"feat/a", "b"}, l.Entries())
}

func TestAnEntryTypedAtOnceThatSpellsAKeyIsTyped(t *testing.T) {
	l := NewTextList(nil, func(EntryCheck) error { return nil }, func(string) string { return "" })

	verdict, cmd := l.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("enter")})

	assert.Equal(t, Stay, settled(l, cmd))
	assert.Equal(t, Stay, verdict)
	assert.Empty(t, l.Entries())
	assert.Equal(t, "enter", l.input.Value())
}
