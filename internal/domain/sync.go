package domain

// WorktreeNode is a managed worktree paired with the branch it was created from.
// It is the input to building a sync plan: the SourceBranch links a node to its
// parent in the dependency tree.
type WorktreeNode struct {
	Branch       string
	Path         string
	SourceBranch string
	IsMain       bool
	// RebaseInProgress is true when the worktree has a rebase stopped mid-way
	// (e.g. left by `wtm sync --keep-conflict`); it cannot be rebased again until
	// resolved.
	RebaseInProgress bool
}

// SyncStep is a single rebase to perform: rebase Branch onto SourceBranch.
type SyncStep struct {
	Branch       string
	Path         string
	SourceBranch string
	// RebaseInProgress mirrors WorktreeNode.RebaseInProgress: the worktree already
	// has a rebase paused mid-way and must be resolved before it can sync.
	RebaseInProgress bool
}

// SyncPlan is the ordered list of rebases for a cascade. Steps are sorted
// topologically so a parent is always rebased before its children.
type SyncPlan struct {
	BaseBranch string
	Steps      []SyncStep
	// BaseTargeted is rules.BaseIsTarget for this cascade; when false the base is
	// neither refreshed nor named.
	BaseTargeted bool
}

// SyncStepStatus is the outcome of a single step in a sync run.
type SyncStepStatus string

const (
	// SyncStatusSynced means the branch was rebased onto its (refreshed) parent.
	SyncStatusSynced SyncStepStatus = "synced"
	// SyncStatusUpToDate means the parent had no new commits; nothing to replay.
	SyncStatusUpToDate SyncStepStatus = "up_to_date"
	// SyncStatusSkippedDirty means the worktree had uncommitted changes and was skipped.
	SyncStatusSkippedDirty SyncStepStatus = "skipped_dirty"
	// SyncStatusSkippedAncestor means an ancestor failed or was skipped, so this
	// branch was skipped to avoid rebasing onto a stale parent.
	SyncStatusSkippedAncestor SyncStepStatus = "skipped_ancestor"
	// SyncStatusDiverged means the local branch and origin/<branch> have both
	// moved (no fast-forward possible), so the branch was left untouched for the
	// user to reconcile manually. Its descendants are skipped.
	SyncStatusDiverged SyncStepStatus = "diverged"
	// SyncStatusConflict means the rebase hit a conflict. By default the rebase is
	// aborted (clean state); under keep-conflict it is left in progress in the
	// worktree for manual resolution (see SyncStepResult.KeptInProgress).
	SyncStatusConflict SyncStepStatus = "conflict"
	// SyncStatusRebaseInProgress means the worktree already has a rebase paused
	// mid-way (e.g. left by a prior keep-conflict run); it is skipped until the user
	// resolves it (git rebase --continue/--abort). Its descendants are skipped too.
	SyncStatusRebaseInProgress SyncStepStatus = "rebase_in_progress"
	// SyncStatusError means the rebase failed for a non-conflict reason.
	SyncStatusError SyncStepStatus = "error"
	// SyncStatusUnknownParent means no parent could be determined (missing metadata).
	SyncStatusUnknownParent SyncStepStatus = "unknown_parent"
	// SyncStatusCancelled means an interrupt reached the branch before or
	// during its rebase: an interrupted rebase is aborted, the branch unchanged.
	SyncStatusCancelled SyncStepStatus = "cancelled"
)

// ParentStatus is the state of a parent no step covers — a branch with no
// worktree, or a worktree left out of the selection. Nothing in the cascade
// refreshes such a parent, so a run inspects it separately.
type ParentStatus string

const (
	// ParentBehind: strictly behind origin/<parent>, left as is.
	ParentBehind ParentStatus = "behind"
	// ParentFastForwarded: advanced to origin/<parent> before its children rebased.
	ParentFastForwarded ParentStatus = "fast_forwarded"
	// ParentDiverged: both refs moved, no fast-forward exists, left untouched.
	ParentDiverged ParentStatus = "diverged"
	// ParentFFFailed: the refresh was asked for and could not happen. Distinct from
	// ParentBehind so the recap names the obstacle instead of re-suggesting the
	// flag already given.
	ParentFFFailed ParentStatus = "ff_failed"
)

// ParentUpdate is one such parent. Only those worth reporting get an entry: one
// already carrying its remote, or without a remote counterpart, produces none.
type ParentUpdate struct {
	Branch string       `json:"branch"`
	Status ParentStatus `json:"status"`
	OldTip string       `json:"old_tip"`
	NewTip string       `json:"new_tip"`
	// Behind counts the commits the local ref lacks; zero for a diverged parent,
	// where no fast-forward distance exists.
	Behind int `json:"behind"`
	// Children are the steps rebased onto this parent.
	Children []string `json:"children"`
	// Detail carries why a fast-forward could not happen (ParentFFFailed).
	Detail string `json:"detail,omitempty"`
}

// SyncStepResult carries the full detail of one step so the recap can reassure
// the user (parent, target commit, before→after, replayed count) before pushing.
type SyncStepResult struct {
	Branch          string         `json:"branch"`
	SourceBranch    string         `json:"source_branch"`
	Path            string         `json:"path,omitempty"`
	Status          SyncStepStatus `json:"status"`
	OldTip          string         `json:"old_tip"`
	NewTip          string         `json:"new_tip"`
	OntoTip         string         `json:"onto_tip"`
	CommitsReplayed int            `json:"commits_replayed"`
	// PushPending is true when the branch has local commits that origin/<branch>
	// lacks (ahead of, or rewritten relative to, its remote) and is eligible for
	// a force-with-lease push. Set for synced and up_to_date branches.
	PushPending bool `json:"push_pending"`
	Pushed      bool `json:"pushed"`
	// ConflictFiles lists the unmerged paths captured when a rebase conflicts. It
	// is populated in both modes (captured before a default abort, or while the
	// rebase is left in progress under keep-conflict).
	ConflictFiles []string `json:"conflict_files,omitempty"`
	// KeptInProgress is true when a conflicting rebase was intentionally left in
	// progress in the worktree (keep-conflict) rather than aborted.
	KeptInProgress bool   `json:"kept_in_progress,omitempty"`
	Detail         string `json:"detail,omitempty"`
}

// SyncResult is the outcome of a full cascade sync.
type SyncResult struct {
	BaseBranch string `json:"base_branch"`
	// BaseTargeted is false when every step rebases onto some other parent: the
	// base is then left untouched rather than moved as a side effect.
	BaseTargeted bool   `json:"base_targeted"`
	BaseUpdated  bool   `json:"base_updated"`
	BaseOldTip   string `json:"base_old_tip"`
	BaseNewTip   string `json:"base_new_tip"`
	// BaseInterrupted says the interrupt came while the base was being refreshed,
	// so an unmoved base may simply not have been read.
	BaseInterrupted bool             `json:"base_interrupted,omitempty"`
	Steps           []SyncStepResult `json:"steps"`
	// SelectedBranches lists the branches the run was asked to sync: the explicit
	// args, or every managed worktree when --all was used. It makes the JSON
	// output self-describing for agents (which branches this cascade covered).
	SelectedBranches []string       `json:"selected_branches"`
	ParentUpdates    []ParentUpdate `json:"parent_updates,omitempty"`
}
