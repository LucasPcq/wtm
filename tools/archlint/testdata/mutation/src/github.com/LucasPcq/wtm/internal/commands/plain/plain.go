package plain

import (
	envsvc "github.com/LucasPcq/wtm/internal/service/env"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

func run() error {
	if err := worktree.Create(); err != nil { // want `worktree\.Create is called from commands/: a worktree-mutating command goes through internal/flow/<cmd>`
		return err
	}
	_ = worktree.List()
	return envsvc.ApplyEnvSync() // want `envsvc\.ApplyEnvSync is called from commands/`
}
