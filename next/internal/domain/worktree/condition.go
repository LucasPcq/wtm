package worktree

import "slices"

// A Condition is the one fact of a worktree that most changes what to do with it first, as one read knows it.
type Condition string

// The conditions of a worktree, the first that applies.
const (
	ConditionNoWorktree    Condition = "no_worktree"                  // a branch of the stack with no worktree
	ConditionUnreadable    Condition = "unreadable"                   // git could not read it
	ConditionMissing       Condition = "missing"                      // its directory is gone
	ConditionRebasing      Condition = Condition(OperationRebase)     // a rebase to finish
	ConditionBisecting     Condition = Condition(OperationBisect)     // a bisect to finish
	ConditionMerging       Condition = Condition(OperationMerge)      // a merge to finish
	ConditionCherryPicking Condition = Condition(OperationCherryPick) // a cherry-pick to finish
	ConditionReverting     Condition = Condition(OperationRevert)     // a revert to finish
	ConditionApplying      Condition = Condition(OperationApply)      // a git am to finish
	ConditionMerged        Condition = "merged"                       // its pull request merged it: only a read that asked GitHub knows it
	ConditionChanged       Condition = "changed"                      // uncommitted changes
	ConditionClean         Condition = "clean"
)

// Conditions returns every Condition, the first that applies first.
func Conditions() []Condition {
	return []Condition{
		ConditionNoWorktree, ConditionUnreadable, ConditionMissing,
		ConditionRebasing, ConditionBisecting, ConditionMerging, ConditionCherryPicking, ConditionReverting, ConditionApplying,
		ConditionMerged, ConditionChanged, ConditionClean,
	}
}

// ConditionParams are what a read knows of a worktree it could read; a read sets ConditionNoWorktree and
// ConditionUnreadable itself, since nothing else of such a worktree is known.
type ConditionParams struct {
	Missing   bool
	Operation Operation // what git stopped in the middle of, "" for nothing
	Merged    bool
	Dirty     bool
}

// ConditionOf is the condition of a worktree a read could read: the first of Conditions that applies.
func ConditionOf(p ConditionParams) Condition {
	switch {
	case p.Missing:
		return ConditionMissing
	case p.Operation != "":
		return Condition(p.Operation)
	case p.Merged:
		return ConditionMerged
	case p.Dirty:
		return ConditionChanged
	}

	return ConditionClean
}

// Operation returns the operation git stopped in the middle of that the condition names, "" when it names none.
func (c Condition) Operation() Operation {
	if !slices.Contains(Operations(), Operation(c)) {
		return ""
	}
	return Operation(c)
}
