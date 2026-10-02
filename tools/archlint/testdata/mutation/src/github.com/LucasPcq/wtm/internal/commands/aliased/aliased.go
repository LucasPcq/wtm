package aliased

import wt "github.com/LucasPcq/wtm/internal/service/worktree"

func run() error {
	return wt.Relocate() // want `wt\.Relocate is called from commands/`
}
