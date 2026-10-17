package components

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestCtrlCAbortsAStandaloneSelect(t *testing.T) {
	list := NewSelectList(NewSelectListParams{Title: "Push?", Items: []SelectItem{{Label: "No", Value: "no"}, {Label: "Yes", Value: "yes"}}})
	for _, filtering := range []bool{false, true} {
		child := list
		child.filtering = filtering
		next, cmd := standaloneModel{child: child}.Update(key(tea.KeyCtrlC))
		m, ok := next.(standaloneModel)
		if !ok || !m.aborted || !isQuit(cmd) {
			t.Errorf("ctrl+c (filtering=%v) did not abort the select", filtering)
		}
	}
}
