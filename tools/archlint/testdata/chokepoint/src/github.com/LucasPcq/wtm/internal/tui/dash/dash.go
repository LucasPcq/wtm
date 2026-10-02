package dash

import w "github.com/LucasPcq/wtm/internal/service/worktree"

var create = w.Create // want `w\.Create is called from tui/`

func run() error { return w.Create() } // want `w\.Create is called from tui/`
