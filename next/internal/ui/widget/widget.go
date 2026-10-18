// Package widget holds the inputs a form's wizard asks a step with: a select list, a multi-select, a text field, a text list, their styles.
package widget

import tea "github.com/charmbracelet/bubbletea"

// A Verdict is what a [Widget] makes of a message: the wizard keeps it, moves on, or goes back.
type Verdict int

// The verdicts of a widget.
const (
	Stay Verdict = iota // the widget is still being answered
	Done                // the widget is answered
	Back                // the user left the widget for the step before
)

// Size is the room the wizard gives a widget to draw in; a Height of 0 leaves it unbounded.
type Size struct {
	Width  int
	Height int
}

// Help is what a widget adds to the wizard's help line.
type Help struct {
	Actions []string // the keys the field adds, as "key action"
	Modal   string   // a help line that replaces the wizard's own, while the widget is in a mode of its own
	Rowless bool     // reports whether the widget has no rows, so no ↑↓ to show
}

// A Widget is the input of one step: the wizard hands it the messages the step receives, draws it in the room it has, and shows its help.
type Widget interface {
	Update(msg tea.Msg) (Verdict, tea.Cmd)
	View(size Size, st *Styles) string
	Help() Help
}
