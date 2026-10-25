package worktree

// A Condition is the one fact of a worktree that most changes what to do with it first, as one read knows it.
type Condition string

// The conditions of a worktree, the first that applies.
const (
	ConditionNoWorktree Condition = "no_worktree" // a branch of the stack with no worktree
	ConditionUnreadable Condition = "unreadable"  // git could not read it
	ConditionMissing    Condition = "missing"     // its directory is gone
	ConditionRebasing   Condition = "rebasing"    // a rebase to finish
	ConditionMerged     Condition = "merged"      // its pull request merged it: only a read that asked GitHub knows it
	ConditionChanged    Condition = "changed"     // uncommitted changes
	ConditionClean      Condition = "clean"
)

// Conditions returns every Condition, the first that applies first.
func Conditions() []Condition {
	return []Condition{ConditionNoWorktree, ConditionUnreadable, ConditionMissing, ConditionRebasing, ConditionMerged, ConditionChanged, ConditionClean}
}

// ConditionParams are what a read knows of a worktree.
type ConditionParams struct {
	NoWorktree bool
	Unreadable bool
	Missing    bool
	Rebasing   bool
	Merged     bool
	Dirty      bool
}

// ConditionOf is the condition of a worktree as a read knows it: the first of Conditions that applies.
func ConditionOf(p ConditionParams) Condition {
	switch {
	case p.NoWorktree:
		return ConditionNoWorktree
	case p.Unreadable:
		return ConditionUnreadable
	case p.Missing:
		return ConditionMissing
	case p.Rebasing:
		return ConditionRebasing
	case p.Merged:
		return ConditionMerged
	case p.Dirty:
		return ConditionChanged
	}

	return ConditionClean
}
