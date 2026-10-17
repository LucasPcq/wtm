package form

import (
	"strings"

	"github.com/LucasPcq/wtm/next/internal/ui/output"
	tea "github.com/charmbracelet/bubbletea"
)

type Confirm struct {
	cursor int
}

func NewConfirm(yes bool) *Confirm {
	if yes {
		return &Confirm{}
	}
	return &Confirm{cursor: 1}
}

func (c *Confirm) Yes() bool {
	return c.cursor == 0
}

func (c *Confirm) Update(msg tea.Msg) (Verdict, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return Stay, nil
	}
	switch key.String() {
	case "up", "k":
		c.cursor = 0
	case "down", "j":
		c.cursor = 1
	case "enter":
		return Done, nil
	case "esc":
		return Back, nil
	}
	return Stay, nil
}

func (c *Confirm) View(size Size, st *Styles) string {
	lines := make([]string, 0, 2)
	for i, answer := range []string{"Yes", "No"} {
		if i != c.cursor {
			lines = append(lines, output.Indent+answer)
			continue
		}
		line := "› " + answer
		lines = append(lines, st.selected.Render(line+strings.Repeat(" ", max(size.Width-len(line)-1, 0))))
	}
	return strings.Join(lines, "\n")
}

func (c *Confirm) Help() Help {
	return Help{}
}
