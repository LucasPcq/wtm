package domain

// TreeNodeStatus holds the orchestration signals rendered on a single tree node.
// All fields are computed locally except PR, which is populated only when PR info
// was fetched (wtm tree --with-prs).
type TreeNodeStatus struct {
	CommitsAhead int  `json:"commits_ahead"`
	IsDirty      bool `json:"is_dirty"`
	// RebaseInProgress is true when the worktree has a rebase paused mid-way (e.g.
	// left by `wtm sync --keep-conflict`) — resolve it with git rebase --continue/--abort.
	RebaseInProgress bool `json:"rebase_in_progress"`
	// NeedsSync is true when the node's parent has advanced past the node, i.e.
	// the parent tip is not an ancestor of the node's branch — the key signal for
	// orchestrating a cascade rebase.
	NeedsSync bool `json:"needs_sync"`
	// InCycle marks a node whose recorded parent chain loops back on itself. The
	// edge is broken in the rendered forest so the tree stays finite.
	InCycle bool `json:"in_cycle"`
	// OriginAhead/OriginBehind/OriginState describe divergence from the node's
	// origin counterpart (cached refs, no fetch). CommitsAhead counts vs the parent
	// — a different referential — so the UI labels them "base" vs "origin".
	OriginAhead  int             `json:"origin_ahead"`
	OriginBehind int             `json:"origin_behind"`
	OriginState  DivergenceState `json:"origin_state"`
	PR           *WorktreeListPR `json:"pr,omitempty"`
	// RunningJobs is how many jobs the run daemon holds for this worktree. Only a
	// surface that asked the daemon fills it; `wtm tree` does not, which is why it
	// is omitted rather than reported as zero.
	RunningJobs int `json:"running_jobs,omitempty"`
}

// TreeNode is one entry in the rendered worktree forest: a managed worktree or a
// virtual node standing in for an unmanaged parent branch (no worktree).
type TreeNode struct {
	Branch string `json:"branch"`
	// Path is the worktree path, empty for a virtual (unmanaged-parent) node.
	Path string `json:"path"`
	// IsMain marks the main worktree (the repository root).
	IsMain bool `json:"is_main"`
	// IsVirtual marks a synthetic node for a parent branch that has no worktree.
	IsVirtual bool           `json:"is_virtual"`
	Status    TreeNodeStatus `json:"status"`
	Children  []TreeNode     `json:"children"`
}

// Forest is the set of root trees rendered by `wtm tree`.
type Forest struct {
	Roots []TreeNode `json:"roots"`
}

// TreeRow is one node of a forest flattened depth-first, carrying the connector
// gutter its position earned it. It is what a renderer draws, so the shape of
// the tree is decided once and never re-derived per surface.
type TreeRow struct {
	Node   TreeNode
	Prefix string
	Depth  int
}

// TreeBadge is one status annotation of a tree node, in the canonical display
// order rules.TreeBadges emits. Every renderer draws the same badges in the same
// order and differs only in how it styles them.
type TreeBadge int

const (
	TreeBadgeVirtual TreeBadge = iota
	TreeBadgeRunning
	TreeBadgePR
	TreeBadgeAhead
	TreeBadgeOrigin
	TreeBadgeRebasing
	TreeBadgeDirty
	TreeBadgeNeedsSync
	TreeBadgeCycle
)
