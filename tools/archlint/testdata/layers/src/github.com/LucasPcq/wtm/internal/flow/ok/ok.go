package ok

import (
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

var _ = worktree.Create

var _ domain.Worktree
