// Package form holds what a command's interactive form is built of: the wizard that walks its steps to a recap, a select list, a multi-select, a text field, a text list, a confirm, their styles.
package form

import tea "github.com/charmbracelet/bubbletea"

// A Verdict is what a [Field] makes of a message: the wizard keeps it, moves on, or goes back.
type Verdict int

// The verdicts of a field.
const (
	Stay Verdict = iota // the field is still being answered
	Done                // the field is answered
	Back                // the user left the field for the step before
)

// Size is the room the wizard gives a field to draw in; a Height of 0 leaves it unbounded.
type Size struct {
	Width  int
	Height int
}

// Help is what a field adds to the wizard's help line.
type Help struct {
	Actions []string // the keys the field adds, as "key action"
	Modal   string   // a help line that replaces the wizard's own, while the field is in a mode of its own
	Rowless bool     // reports whether the field has no rows, so no ↑↓ to show
}

// A Field is the input of one step: the wizard hands it the messages the step receives, draws it in the room it has, and shows its help.
type Field interface {
	Update(msg tea.Msg) (Verdict, tea.Cmd)
	View(size Size, st *Styles) string
	Help() Help
}
