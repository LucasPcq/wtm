package widget

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestAHookIsAddedWithItsFolderAndItsSwitch(t *testing.T) {
	l := NewHookList([]project.Hook{{Cmd: "pnpm install"}}, func([]project.Hook) error { return nil })

	press(l, "down", "enter", "make db", "tab", "packages/db", "tab", "space", "enter")
	verdict := press(l, "down", "down", "enter")

	assert.Equal(t, Done, verdict)
	assert.Equal(t, []project.Hook{{Cmd: "pnpm install"}, {Cmd: "make db", Cwd: "packages/db", ContinueOnError: true}}, l.Hooks())
}

func TestAHookIsMovedRemovedAndEdited(t *testing.T) {
	l := NewHookList([]project.Hook{{Cmd: "a"}, {Cmd: "b"}, {Cmd: "c"}}, func([]project.Hook) error { return nil })

	l.Update(tea.KeyMsg{Type: tea.KeyShiftDown})
	press(l, "down", "d", "up", "enter", "backspace", "z", "enter")

	assert.Equal(t, []project.Hook{{Cmd: "b"}, {Cmd: "z"}}, l.Hooks(), "a moved down, c removed, a edited into z")
}

func TestAHookWithNoCommandIsNotSaved(t *testing.T) {
	l := NewHookList(nil, func([]project.Hook) error { return nil })

	press(l, "enter", "enter")

	assert.Contains(t, plain(l.View(Size{Width: 80}, plainStyles())), "type a command")
	press(l, "esc")
	assert.Empty(t, l.Hooks())
}

func TestARefusedListStaysWithItsProblem(t *testing.T) {
	l := NewHookList([]project.Hook{{Cmd: "x", Cwd: "/srv"}}, func([]project.Hook) error { return errors.New("on_clean[0] runs outside the repository") })

	verdict := press(l, "down", "down", "enter")

	assert.Equal(t, Stay, verdict)
	assert.Contains(t, plain(l.View(Size{Width: 80}, plainStyles())), "runs outside the repository")
}

func TestEscLeavesTheListForTheStepBefore(t *testing.T) {
	assert.Equal(t, Back, press(NewHookList(nil, nil), "esc"))
}
