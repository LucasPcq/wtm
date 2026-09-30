package rules

import (
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

// FocusExitParams holds the two presses a way out is made of.
type FocusExitParams struct {
	Previous time.Time
	Now      time.Time
}

// ExitsFocus reports whether the exit key just landed close enough behind its
// own previous press to mean the reader wants the keyboard back. A lone press
// never does: the job is entitled to it.
func ExitsFocus(params FocusExitParams) bool {
	if params.Previous.IsZero() {
		return false
	}
	gap := params.Now.Sub(params.Previous)
	return gap >= 0 && gap <= domain.RunViewFocusExitWindow
}
