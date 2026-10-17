package components

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
)

func sendHook(m HookListModel, msgs ...tea.Msg) HookListModel {
	for _, msg := range msgs {
		m, _ = m.Update(msg)
	}
	return m
}

func typeHook(m HookListModel, s string) HookListModel {
	for _, r := range s {
		m = sendHook(m, runeKey(r))
	}
	return m
}

func TestHookListAddAndDone(t *testing.T) {
	m := NewHookList(NewHookListParams{})

	// cursor on "+ Add" (index 0 when empty) → enter starts the edit form.
	m = sendHook(m, key(tea.KeyEnter))
	if !m.editing {
		t.Fatal("enter on Add row should start editing")
	}

	m = typeHook(m, "pnpm install")
	m = sendHook(m, key(tea.KeyEnter)) // save
	if m.editing {
		t.Fatal("enter should save and leave edit mode")
	}
	if len(m.Hooks()) != 1 || m.Hooks()[0].Cmd != "pnpm install" {
		t.Fatalf("expected one hook 'pnpm install', got %+v", m.Hooks())
	}

	// Navigate to Done and finish.
	m = sendHook(m, key(tea.KeyDown), key(tea.KeyDown), key(tea.KeyEnter))
	if !m.Done() {
		t.Error("enter on Done row should finish the step")
	}
}

func TestHookListEmptyCmdRejected(t *testing.T) {
	m := NewHookList(NewHookListParams{})
	m = sendHook(m, key(tea.KeyEnter)) // start add
	m = sendHook(m, key(tea.KeyEnter)) // save with empty cmd
	if !m.editing {
		t.Error("empty command should keep the form open")
	}
	if m.formErr == "" {
		t.Error("empty command should set a validation error")
	}
	if len(m.Hooks()) != 0 {
		t.Error("no hook should be added on validation failure")
	}
}

func TestHookListEditWithCwd(t *testing.T) {
	m := NewHookList(NewHookListParams{Hooks: []domain.HookCommand{{Cmd: "pnpm install"}}})

	// cursor on the hook (0) → edit.
	m = sendHook(m, key(tea.KeyEnter))
	if !m.editing {
		t.Fatal("enter on a hook should edit it")
	}
	// move to cwd field and type a value.
	m = sendHook(m, key(tea.KeyTab))
	m = typeHook(m, "packages/api")
	m = sendHook(m, key(tea.KeyEnter)) // save

	hooks := m.Hooks()
	if len(hooks) != 1 || hooks[0].Cwd != "packages/api" {
		t.Fatalf("expected cwd packages/api, got %+v", hooks)
	}
}

func TestHookListDelete(t *testing.T) {
	m := NewHookList(NewHookListParams{Hooks: []domain.HookCommand{{Cmd: "a"}, {Cmd: "b"}}})
	m = sendHook(m, runeKey('d')) // delete the first
	if len(m.Hooks()) != 1 || m.Hooks()[0].Cmd != "b" {
		t.Fatalf("expected only 'b' left, got %+v", m.Hooks())
	}
}

func TestHookListReorder(t *testing.T) {
	m := NewHookList(NewHookListParams{Hooks: []domain.HookCommand{{Cmd: "a"}, {Cmd: "b"}}})
	m = sendHook(m, key(tea.KeyShiftDown)) // move 'a' down
	got := m.Hooks()
	if got[0].Cmd != "b" || got[1].Cmd != "a" {
		t.Fatalf("expected order b,a, got %+v", got)
	}
}
