package process

import (
	"github.com/LucasPcq/wtm/internal/service/proxy"
	"github.com/LucasPcq/wtm/internal/service/worktree" // want `internal/service/process must not import "github.com/LucasPcq/wtm/internal/service/worktree" — undeclared service edge`
)

var Placeholder = 0

var _ = proxy.Route

var _ = worktree.Create
