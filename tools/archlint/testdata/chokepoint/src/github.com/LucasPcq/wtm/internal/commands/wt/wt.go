package wt

import (
	envsvc "github.com/LucasPcq/wtm/internal/service/env"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

func run() error {
	_ = worktree.List()
	if err := worktree.SetIsolation(); err != nil { // want `worktree\.SetIsolation is called from commands/: a worktree-mutating command goes through internal/flow/<cmd>`
		return err
	}
	return envsvc.ApplyEnvSync() // want `envsvc\.ApplyEnvSync is called from commands/`
}
