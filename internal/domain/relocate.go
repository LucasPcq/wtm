package domain

// RelocateStatus is the planned action or executed outcome for one worktree in
// a relocate run. The plan uses the intent values (move/adopt/noop) and the skip
// reasons; the result reuses the skip reasons and adds the executed outcomes.
type RelocateStatus string

const (
	// RelocateStatusMove means the worktree will be moved to its target path.
	RelocateStatusMove RelocateStatus = "move"
	// RelocateStatusAdopt means the worktree is already at its target path but
	// lacks wtm metadata, so a meta.json will be created in place.
	RelocateStatusAdopt RelocateStatus = "adopt"
	// RelocateStatusNoop means the worktree is managed and already in place.
	RelocateStatusNoop RelocateStatus = "noop"
	// RelocateStatusSkippedDirty means the worktree has uncommitted changes and
	// was not moved (use --force to move anyway).
	RelocateStatusSkippedDirty RelocateStatus = "skipped_dirty"
	// RelocateStatusSkippedLocked means the worktree is locked and was not moved
	// (use --force to move anyway).
	RelocateStatusSkippedLocked RelocateStatus = "skipped_locked"
	// RelocateStatusBlockedDest means the target path is already occupied by an
	// unrelated directory; the move is blocked and --force does not override it.
	RelocateStatusBlockedDest RelocateStatus = "blocked_dest"
	// RelocateStatusBlockedJobs means jobs run in the worktree: they are keyed
	// on its path, so moving it would orphan them. --force does not lift it.
	RelocateStatusBlockedJobs RelocateStatus = "blocked_jobs"
	// RelocateStatusMoved means the worktree was moved to its target path.
	RelocateStatusMoved RelocateStatus = "moved"
	// RelocateStatusAdopted means a meta.json was created for the worktree in place.
	RelocateStatusAdopted RelocateStatus = "adopted"
	// RelocateStatusMovedAdopted means the worktree was both moved and adopted.
	RelocateStatusMovedAdopted RelocateStatus = "moved_adopted"
	// RelocateStatusError means the move or adoption failed during execution.
	RelocateStatusError RelocateStatus = "error"
)

// RelocateStep is a single planned worktree relocation/adoption.
type RelocateStep struct {
	Branch   string         `json:"branch"`
	FromPath string         `json:"from_path"`
	ToPath   string         `json:"to_path"`
	Status   RelocateStatus `json:"status"`
	// Adopt is true when the worktree lacks wtm metadata and a meta.json will be
	// created for it (independent of whether it is also moved).
	Adopt bool `json:"adopt"`
	// Parent is the source branch recorded when adopting (defaults to the base branch).
	Parent string `json:"parent,omitempty"`
}

// RelocatePlan is the full set of planned relocations for a repo.
type RelocatePlan struct {
	BasePath   string         `json:"base_path"`
	BaseBranch string         `json:"base_branch"`
	Steps      []RelocateStep `json:"steps"`
}

// RelocateStepResult is the outcome of one executed relocate step.
type RelocateStepResult struct {
	Branch   string         `json:"branch"`
	FromPath string         `json:"from_path"`
	ToPath   string         `json:"to_path"`
	Status   RelocateStatus `json:"status"`
	Parent   string         `json:"parent,omitempty"`
	Detail   string         `json:"detail,omitempty"`
}

// RelocateResult is the outcome of a full relocate run.
type RelocateResult struct {
	BasePath        string               `json:"base_path"`
	BasePathUpdated bool                 `json:"base_path_updated"`
	Steps           []RelocateStepResult `json:"steps"`
}
