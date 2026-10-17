package form

import (
	"strings"

	"github.com/LucasPcq/wtm/next/internal/surface/output"
	tea "github.com/charmbracelet/bubbletea"
)

type confirm struct {
	cursor int
}

func newConfirm(yes bool) *confirm {
	if yes {
		return &confirm{}
	}
	return &confirm{cursor: 1}
}

func (c *confirm) yes() bool {
	return c.cursor == 0
}

func (c *confirm) update(msg tea.Msg) (verdict, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return stay, nil
	}
	switch key.String() {
	case "up", "k":
		c.cursor = 0
	case "down", "j":
		c.cursor = 1
	case "enter":
		return done, nil
	case "esc":
		return back, nil
	}
	return stay, nil
}

func (c *confirm) view(size size) string {
	lines := make([]string, 0, 2)
	for i, answer := range []string{"Yes", "No"} {
		if i != c.cursor {
			lines = append(lines, output.Indent+answer)
			continue
		}
		line := "› " + answer
		lines = append(lines, selected.Render(line+strings.Repeat(" ", max(size.Width-len(line)-1, 0))))
	}
	return strings.Join(lines, "\n")
}

func (c *confirm) help() help {
	return help{}
}
