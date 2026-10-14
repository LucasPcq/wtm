package worktree

import (
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// RebasingParams is what a worktree's admin directory says of its detached
// HEAD: the head-name a rebase in progress leaves, one per backend found.
type RebasingParams struct {
	DirGone   bool
	HeadNames []string
}

// RebasingBranch is the branch a rebase in progress will move, empty for a
// HEAD merely detached. A worktree whose directory is gone has nothing left to
// say: it is merely detached.
func RebasingBranch(params RebasingParams) string {
	if params.DirGone {
		return ""
	}
	for _, headName := range params.HeadNames {
		if branch := strings.TrimPrefix(strings.TrimSpace(headName), domain.LocalRefPrefix); branch != "" {
			return branch
		}
	}
	return ""
}
