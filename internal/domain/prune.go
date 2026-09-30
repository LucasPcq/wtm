package domain

// PruneCandidate is a worktree selected for removal by `wtm prune`, tagged with
// the first filter reason that matched it (one of the PruneReason* constants).
type PruneCandidate struct {
	Branch  string `json:"branch"`
	Path    string `json:"path"`
	Reason  string `json:"reason"`
	IsDirty bool   `json:"is_dirty"`
	// UnsafeReason is set (to a PruneSkip* constant: dirty/unpushed/open_pr) when
	// the candidate is only in the selection because --force overrode a safety
	// check. Empty means the candidate is safe to remove. Consumed by the picker
	// (opt-in display) and FinalizePrunePlan (drop unless the user confirms force).
	UnsafeReason string `json:"unsafe_reason,omitempty"`
	SourceBranch string `json:"source_branch,omitempty"`
}

// PruneSkip records a worktree that matched a filter but was deliberately not
// removed, with the reason (one of the PruneSkip* constants).
type PruneSkip struct {
	Branch string `json:"branch"`
	Reason string `json:"reason"`
}

// PrunePlan is the computed, side-effect-free outcome of classification: which
// worktrees would be removed, which were skipped, and the reparenting of any
// surviving children onto their grandparent.
type PrunePlan struct {
	Selected  []PruneCandidate
	Skipped   []PruneSkip
	Reparents []ReparentResult
}

// PruneResult is the outcome of an executed (or dry-run) prune.
type PruneResult struct {
	Pruned     []PruneCandidate `json:"pruned"`
	Reparented []ReparentResult `json:"reparented"`
	// Orphaned lists children left pointing at a removed parent because the user
	// declined reparenting at the confirmation step.
	Orphaned []ReparentResult `json:"orphaned"`
	Skipped  []PruneSkip      `json:"skipped"`
	DryRun   bool             `json:"dry_run"`
	// Namespaces is what became of the data each pruned worktree held in the
	// shared services.
	Namespaces []NamespaceOutcome `json:"namespaces"`
	// Failed is the worktree the run stopped on. The ones after it were not
	// touched, and neither was its data.
	Failed *PruneFailure `json:"failed,omitempty"`
}

type PruneFailure struct {
	Branch string `json:"branch"`
	Path   string `json:"path"`
	Error  string `json:"error"`
}

// PruneParams holds inputs for planning and executing a prune.
type PruneParams struct {
	ProjectDir string
	StateDir   string
	Config     Config
	BaseBranch string
	// Reason filters. When none are set the command defaults to all three.
	Merged bool
	Closed bool
	Gone   bool
	// NoFetch skips the `git fetch --prune` that the Gone filter needs to be
	// accurate (the caller has already fetched).
	NoFetch bool
	// Force allows removing dirty worktrees; without it they are skipped.
	Force  bool
	DryRun bool
}
