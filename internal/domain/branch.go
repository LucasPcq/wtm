package domain

// DivergenceState classifies how a local branch relates to its origin
// remote-tracking counterpart, derived from the ahead/behind commit counts.
type DivergenceState int

const (
	// DivergenceUnknown means the branch has no origin counterpart (local-only)
	// or the comparison was not computed; no divergence badge is shown.
	DivergenceUnknown DivergenceState = iota
	// DivergenceUpToDate means local and origin point at the same commit.
	DivergenceUpToDate
	// DivergenceBehind means origin has commits the local branch lacks
	// (fast-forward possible).
	DivergenceBehind
	// DivergenceAhead means the local branch has commits not yet on origin.
	DivergenceAhead
	// DivergenceDiverged means each side has commits the other lacks.
	DivergenceDiverged
)

// AheadBehind holds the commit counts of a local branch relative to its origin
// counterpart: Ahead commits exist only locally, Behind commits exist only on
// origin.
type AheadBehind struct {
	Ahead  int
	Behind int
}

// BranchTargetState classifies the branch a worktree is about to be created on:
// whether git will create it, reuse an existing one, or refuse because another
// worktree already holds it.
type BranchTargetState int

const (
	// BranchTargetNew means no local branch of that name exists — git creates it
	// with -b from the source branch.
	BranchTargetNew BranchTargetState = iota
	// BranchTargetExisting means a local branch of that name exists and no
	// worktree holds it, so the new worktree checks it out as-is. The source
	// branch is then only the sync parent recorded in metadata, not a start-point.
	BranchTargetExisting
	// BranchTargetCheckedOut means a local branch exists but another worktree (or
	// the main one) already has it checked out — git allows only one.
	BranchTargetCheckedOut
)

// BranchTarget describes the branch a worktree is about to be created on.
type BranchTarget struct {
	State  BranchTargetState
	Branch string
	// WorktreePath is the worktree already holding the branch, set only for
	// BranchTargetCheckedOut.
	WorktreePath string
	// Origin is how an existing local branch diverges from origin/<branch>;
	// DivergenceUnknown when the branch is new or has no origin counterpart.
	Origin      DivergenceState
	AheadBehind AheadBehind
}

// BranchCandidate is a branch offered in a picker as a worktree start-point or
// parent. Name is the git ref used as the value: a bare local name ("feature")
// or a remote-tracking ref ("origin/feature"). IsRemote tags the latter so the
// UI can badge it and group remotes separately. Ahead/Behind/State describe how
// a local branch diverges from its origin counterpart (zero/Unknown when there
// is none) so the UI can warn before a worktree starts from a stale ref.
type BranchCandidate struct {
	Name     string
	IsRemote bool
	Ahead    int
	Behind   int
	State    DivergenceState
}

// FastForwardStatus is what became of one branch in a fast-forward run.
type FastForwardStatus int

const (
	FFUpToDate FastForwardStatus = iota
	FFAdvanced
	FFDiverged
	FFNoUpstream
	FFFailed
)

// FastForwardCheck is one branch's state against origin, gathered in a single
// network round trip so a recap and the run that follows it read the same facts.
type FastForwardCheck struct {
	Branch string
	// WorktreePath is empty when the branch is checked out nowhere.
	WorktreePath string
	IsDirty      bool
	Ahead        int
	Behind       int
	State        DivergenceState
	HasUpstream  bool
}

type FastForwardResult struct {
	Branch string            `json:"branch"`
	Status FastForwardStatus `json:"-"`
	Label  string            `json:"status"`
	OldTip string            `json:"old_tip"`
	NewTip string            `json:"new_tip"`
	Behind int               `json:"behind"`
	// Detail carries git's own message when Status is FFFailed.
	Detail string `json:"detail,omitempty"`
}

// FastForwardBlocker is a safety refusal a surface may have lifted one at a
// time. Only refusals that --force may lift are ever built as one.
type FastForwardBlocker struct {
	Key   string
	Label string
}

// Upstream is what a local branch tracks, as `git for-each-ref` reads it: empty
// fields for a branch with no upstream. Gone is git's "[gone]": an upstream
// configured whose remote-tracking ref no longer exists.
type Upstream struct {
	Remote      string
	RemoteRef   string
	TrackingRef string
	Gone        bool
}
