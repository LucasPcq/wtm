package form

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSpaceTogglesTheRowUnderTheCursor(t *testing.T) {
	m := NewMultiSelect(MultiSelectParams{Options: picks()})

	pressed(m, "space", "down", "down", "space", "space", "up", "space")

	assert.Equal(t, []string{"a.txt", "b.txt"}, m.Values())
}

func TestAPicksEveryShownRow(t *testing.T) {
	m := NewMultiSelect(MultiSelectParams{Options: picks()})

	pressed(m, "a")

	assert.Equal(t, []string{"a.txt", "b.txt", "notes.md"}, m.Values())
}

func TestAFilterNarrowsWhatAPicks(t *testing.T) {
	m := NewMultiSelect(MultiSelectParams{Options: picks()})

	pressed(m, "/", "t", "x", "t", "esc", "a")

	assert.Equal(t, []string{"a.txt", "b.txt"}, m.Values())
	assert.Contains(t, plain(m.View(Size{Width: 40}, plainStyles())), "/ txt")
	pressed(m, "esc")
	assert.Contains(t, plain(m.View(Size{Width: 40}, plainStyles())), "notes.md")
}

func TestEnterRefusesAnEmptySelectionWithTheCallersMessage(t *testing.T) {
	m := NewMultiSelect(MultiSelectParams{Options: picks(), Empty: "pick at least one file"})

	assert.Equal(t, Stay, pressed(m, "enter"))
	assert.Contains(t, plain(m.View(Size{Width: 40}, plainStyles())), "✗ Pick at least one file")

	assert.Equal(t, Done, pressed(m, "space", "enter"))
	assert.NotContains(t, plain(m.View(Size{Width: 40}, plainStyles())), "Pick at least one file")
}

func TestEscGoesBack(t *testing.T) {
	m := NewMultiSelect(MultiSelectParams{Options: picks()})

	assert.Equal(t, Back, pressed(m, "esc"))
}

func TestTheViewMarksEachPickAndItsTag(t *testing.T) {
	m := NewMultiSelect(MultiSelectParams{Options: picks()})
	pressed(m, "down", "space")

	assert.Equal(t, "   [ ] a.txt                  modified\n▌› [x] b.txt                 untracked\n   [ ] notes.md", plain(m.View(Size{Width: 40}, plainStyles())))
	assert.Equal(t, Help{Actions: []string{"space toggle", "a all", "/ filter"}}, m.Help())
	pressed(m, "/")
	assert.Equal(t, Help{Modal: pickFilterHelp}, m.Help())
}

func TestTheViewScrollsToTheCursor(t *testing.T) {
	m := NewMultiSelect(MultiSelectParams{Options: picks()})
	pressed(m, "down", "down")

	assert.Equal(t, "   [ ] b.txt                 untracked\n▌› [ ] notes.md", plain(m.View(Size{Width: 40, Height: 2}, plainStyles())))
}
