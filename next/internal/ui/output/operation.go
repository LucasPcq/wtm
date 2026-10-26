package output

import "github.com/LucasPcq/wtm/next/internal/domain/worktree"

var operationWords = map[worktree.Operation]string{
	worktree.OperationRebase:     "rebasing",
	worktree.OperationBisect:     "bisecting",
	worktree.OperationMerge:      "merging",
	worktree.OperationCherryPick: "cherry-picking",
	worktree.OperationRevert:     "reverting",
	worktree.OperationApply:      "applying patches",
}

// OperationWord is how the command line names what git stopped in the middle of in a worktree, "" for none.
func OperationWord(operation worktree.Operation) string {
	return operationWords[operation]
}
