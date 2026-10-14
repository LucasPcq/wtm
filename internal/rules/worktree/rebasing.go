package worktree

import (
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// RebasingParams holds the head-name a rebase in progress leaves in a worktree's admin directory, one per backend found.
type RebasingParams struct {
	DirGone   bool
	HeadNames []string
}

// RebasingBranch reads a worktree whose directory is gone as merely detached: it has nothing left to say.
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
