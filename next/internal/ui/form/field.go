// Package form holds what a command's interactive form is built of: the wizard that walks its steps to a recap, a select list, a multi-select, a text list, a confirm, their styles.
package form

import tea "github.com/charmbracelet/bubbletea"

type Verdict int

const (
	Stay Verdict = iota
	Done
	Back
)

type Size struct {
	Width  int
	Height int
}

type Help struct {
	Actions []string
	Modal   string
	Rowless bool
}

type Field interface {
	Update(msg tea.Msg) (Verdict, tea.Cmd)
	View(size Size, st *Styles) string
	Help() Help
}
