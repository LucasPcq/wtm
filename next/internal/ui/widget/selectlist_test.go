package widget

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestTextTypedOrPastedAtOnceFiltersTheList(t *testing.T) {
	for name, key := range map[string]tea.KeyMsg{
		"typed":  {Type: tea.KeyRunes, Runes: []rune("logo")},
		"pasted": {Type: tea.KeyRunes, Runes: []rune("logo"), Paste: true},
	} {
		t.Run(name, func(t *testing.T) {
			l := NewSelectList(SelectParams{Options: branches()})

			l.Update(keyOf("/"))
			l.Update(key)

			assert.Equal(t, "feat/logout", l.Value())
			assert.NotContains(t, plain(l.View(Size{Width: 40}, plainStyles())), "feat/login")
		})
	}
}

func TestBackspaceInTheFilterRemovesAWholeCharacter(t *testing.T) {
	l := NewSelectList(SelectParams{Options: branches()})

	l.Update(keyOf("/"))
	l.Update(keyOf("café"))
	l.Update(keyOf("backspace"))

	assert.Equal(t, "fix/café", l.Value())
	assert.Contains(t, plain(l.View(Size{Width: 40}, plainStyles())), "/ caf\n")
}

func TestTextTypedAtOnceThatSpellsAKeyIsFilteredNotPressed(t *testing.T) {
	l := NewSelectList(SelectParams{Options: branches()})

	l.Update(keyOf("/"))
	l.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("esc")})

	assert.True(t, l.Filtering())
	assert.Contains(t, plain(l.View(Size{Width: 40}, plainStyles())), "/ esc")
}

func TestASlashTypedAtOnceWithTextFiltersTheListByIt(t *testing.T) {
	l := NewSelectList(SelectParams{Options: branches()})

	l.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/logo")})

	assert.True(t, l.Filtering())
	assert.Equal(t, "feat/logout", l.Value())
}
