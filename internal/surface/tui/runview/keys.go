package runview

import "github.com/LucasPcq/wtm/internal/domain"

const (
	keyQuit      = domain.KeyQuit
	keyInterrupt = "ctrl+c"
	keyRefresh   = domain.KeyRefresh
	keyFilter    = "/"
	keyEscape    = "esc"
	keyEnter     = "enter"
	keyBackspace = "backspace"
	keyUp        = "up"
	keyDown      = "down"
	keyVimUp     = "k"
	keyVimDown   = "j"
	keyPageUp    = "pgup"
	keyPageDown  = "pgdown"
	keyScrollUp  = "shift+up"
	keyScrollDwn = "shift+down"
	keyLive      = "G"
	keyOpenURL   = domain.KeyOpenURL
	keyReach     = domain.RunViewReachKey
)
