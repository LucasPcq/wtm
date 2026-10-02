package process

import (
	"github.com/LucasPcq/wtm/internal/config" // want `the daemon must not import "github.com/LucasPcq/wtm/internal/config"`
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/service/proxy"
	"github.com/LucasPcq/wtm/internal/service/worktree" // want `the daemon must not import "github.com/LucasPcq/wtm/internal/service/worktree"`
)

var _ = proxy.Route

var _ = config.Load

var _ = worktree.Create

func dirs() (string, error) {
	global, err := infra.GlobalDir()
	if err != nil {
		return "", err
	}
	return infra.Toplevel(global) // want `the daemon calls infra\.Toplevel: only GlobalDir is allow-listed`
}

var _ infra.Paths // want `the daemon calls infra\.Paths`
