package form_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	tea "github.com/charmbracelet/bubbletea"
)

var size = tea.WindowSizeMsg{Width: 80, Height: 30}

func open(t *testing.T, params form.Params) *formtest.Driver {
	t.Helper()
	return start(form.New(t.Context(), params))
}

func start(w *form.Wizard) *formtest.Driver {
	return formtest.Start(w, size)
}
