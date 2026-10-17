package dashboard

import "github.com/LucasPcq/wtm/internal/domain"

// Every clickable zone has a key here: the mouse is a convenience, the keyboard
// stays complete.
const (
	keyQuit         = domain.KeyQuit
	keyInterrupt    = "ctrl+c"
	keyHelp         = domain.KeyHelp
	keyRefresh      = domain.KeyRefresh
	keyToggleOutput = domain.KeyToggleOutput
	keyEscape       = "esc"
	keyEnter        = "enter"
	keyTab          = "tab"
	keyShiftTab     = "shift+tab"
	keyUp           = "up"
	keyDown         = "down"
	keyLeft         = "left"
	keyRight        = "right"
	keyVimUp        = "k"
	keyVimDown      = "j"
	keyVimLeft      = "h"
	keyVimRight     = "l"
	keyTop          = "g"
	keyBottom       = "G"
	keyPageUp       = "pgup"
	keyPageDown     = "pgdown"
	keyNew          = domain.KeyNew
	keyMenu         = domain.KeyMenu
	keyActions      = domain.KeyActions
	keyOpenPR       = domain.KeyOpenPR
	keyRunLogs      = domain.KeyRunLogs
	keyOpenAddress  = domain.KeyOpenAddress
	keyFastForward  = domain.KeyFastForward
	keySpace        = " "
	keyOutputUp     = "shift+up"
	keyOutputDown   = "shift+down"
)
