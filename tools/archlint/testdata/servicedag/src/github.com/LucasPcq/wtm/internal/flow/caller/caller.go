package caller

import (
	"github.com/LucasPcq/wtm/internal/service/branch"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

var _, _ = branch.List, worktree.Create
