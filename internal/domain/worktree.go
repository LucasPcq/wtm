package domain

import (
	"io"
	"time"
)

// Worktree represents a git worktree managed by wtm.
type Worktree struct {
	Name   string
	Path   string
	Branch string
}

// WorktreeMetadata is written to <state-dir>/worktrees/<branch>/meta.json
// for every worktree managed by wtm.
type WorktreeMetadata struct {
	SourceBranch string      `json:"source_branch"`
	CreatedAt    string      `json:"created_at"`
	EnvStrategy  EnvStrategy `json:"env_strategy"`
	// Ordinal is the worktree's stable number, what every port and resource name
	// is derived from. Zero means unallocated: the main worktree is ordinal 0 by
	// definition and never gets a meta.json of its own.
	Ordinal int `json:"ordinal,omitempty"`
	// Tenants names the shared services this worktree has carved a tenant out
	// of. It is the only durable record that one exists: a claim on a shared
	// service goes with a `run stop`, and without this a clean would either give
	// back a tenant that was never created or leak one that was. It lives here
	// because the file is removed with the worktree it describes.
	Tenants []string `json:"tenants,omitempty"`
}

// WorktreeStatus holds the display state of a worktree for wtm ls.
type WorktreeStatus struct {
	Branch       string
	Path         string
	IsParent     bool
	IsDirty      bool
	CommitsAhead int
	CreatedAt    time.Time
	// RebaseInProgress is true when the worktree has a rebase paused mid-way (e.g.
	// left by `wtm sync --keep-conflict`). Its branch is recovered from the paused
	// rebase; the worktree is also dirty, but this is the more precise signal.
	RebaseInProgress bool
	// OriginAhead/OriginBehind/OriginState describe how the worktree's branch
	// diverges from its origin counterpart (from cached remote-tracking refs, no
	// fetch). OriginState is DivergenceUnknown when there is no counterpart.
	// CommitsAhead counts commits vs the base/parent branch — a different
	// referential — so the UI labels them "base" vs "origin" to disambiguate.
	OriginAhead  int
	OriginBehind int
	OriginState  DivergenceState
}

// WorktreeListEntry is the JSON-serializable projection of a worktree for the
// `list --output json` payload.
type WorktreeListEntry struct {
	Branch           string              `json:"branch"`
	Path             string              `json:"path"`
	IsParent         bool                `json:"is_parent"`
	IsDirty          bool                `json:"is_dirty"`
	RebaseInProgress bool                `json:"rebase_in_progress"`
	CommitsAhead     int                 `json:"commits_ahead"`
	CreatedAt        time.Time           `json:"created_at"`
	Origin           *WorktreeListOrigin `json:"origin"`
	PR               *WorktreeListPR     `json:"pr"`
	Services         []string            `json:"services"`
}

// WorktreeListPR is the nested PR summary embedded in WorktreeListEntry.
type WorktreeListPR struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
	State  string `json:"state"`
}

// WorktreeListOrigin is the nested origin-divergence summary embedded in
// WorktreeListEntry. It is null when the branch has no origin counterpart.
type WorktreeListOrigin struct {
	Ahead  int    `json:"ahead"`
	Behind int    `json:"behind"`
	State  string `json:"state"`
}

// GitWorktree represents a worktree entry from git worktree list.
type GitWorktree struct {
	Path   string
	Branch string
	IsMain bool
	Locked bool
	// RebaseInProgress is true when the worktree has a rebase stopped mid-way
	// (e.g. left in progress by `wtm sync --keep-conflict`). Its HEAD is detached,
	// so Branch is recovered from the in-progress rebase's original branch.
	RebaseInProgress bool
}

// CreateParams holds all inputs needed to create a new worktree.
type CreateParams struct {
	ProjectDir string
	StateDir   string
	Branch     string
	// FromBranch is the git start-point the worktree is created from.
	FromBranch string
	// SourceBranch is the parent recorded in metadata and used as the rebase
	// target by `wtm sync`. When empty, it defaults to FromBranch. It diverges
	// from FromBranch for PR checkouts, where the worktree content is the PR head
	// but the parent is the PR base branch.
	SourceBranch    string
	Config          Config
	EnvFromOverride string
	IfNotExists     bool
	// SkipHooks leaves on_create hooks unrun so the caller can execute them as a
	// separate, titled phase (used by `create` for its phased output). Callers that
	// want hooks to run inline (extract, checkout) leave it false.
	SkipHooks bool
}

// CreateHooksParams holds inputs for running on_create hooks as a standalone phase,
// after the worktree exists.
type CreateHooksParams struct {
	ProjectDir   string
	StateDir     string
	WorktreePath string
	Branch       string
	FromBranch   string
	Hooks        []HookCommand
	// Output receives the hook output as it is produced; nil keeps stderr.
	Output io.Writer
}

// CreateResult holds the output of a successful worktree creation.
type CreateResult struct {
	Branch        string           `json:"branch"`
	Path          string           `json:"path"`
	Metadata      WorktreeMetadata `json:"metadata"`
	AlreadyExists bool             `json:"already_exists"`
	// ExistingBranch reports that the worktree checked out a local branch that
	// already existed instead of creating one; the source branch was then only
	// recorded as the sync parent, not used as a start-point.
	ExistingBranch bool `json:"existing_branch"`
	// OriginState is the reused branch's divergence from origin, using the same
	// labels as `list` and `tree`. Empty when the branch was created.
	OriginState string `json:"origin_state,omitempty"`
	// OriginAhead/OriginBehind are the reused branch's commit counts vs its origin
	// counterpart, backing the human-readable reuse note. Zero when the branch was
	// created (OriginState empty) or up to date.
	OriginAhead  int `json:"origin_ahead,omitempty"`
	OriginBehind int `json:"origin_behind,omitempty"`
}

// CleanParams holds inputs for cleaning a worktree.
type CleanParams struct {
	ProjectDir string
	StateDir   string
	Branch     string
	Force      bool
	// BaseBranch is the fallback parent for orphaned children when the cleaned
	// worktree has no recorded parent of its own.
	BaseBranch string
	Config     Config
	// SkipHooks leaves on_clean hooks unrun so the caller can execute them as a
	// separate, titled phase before removal (used by `clean` for its phased output).
	SkipHooks bool
}

// CleanHooksParams holds inputs for running on_clean hooks as a standalone phase,
// before the worktree directory is removed.
type CleanHooksParams struct {
	ProjectDir   string
	StateDir     string
	WorktreePath string
	Branch       string
	Hooks        []HookCommand
	// Output receives the hook output as it is produced; nil keeps stderr.
	Output io.Writer
}

// ForceCleanParams holds inputs for the forced worktree recovery: delete the
// directory with `sudo rm -rf`, prune the stale git metadata, then delete the
// local branch. Used when `git worktree remove` failed on undeletable files.
type ForceCleanParams struct {
	ProjectDir string
	StateDir   string
	Path       string
	Branch     string
	Force      bool
}

// ReparentBatchParams holds inputs for reparenting one or more worktrees onto the
// same new parent in a single pass. A single-element Branches is the ordinary
// one-worktree reparent.
type ReparentBatchParams struct {
	ProjectDir string
	StateDir   string
	Branches   []string
	NewParent  string
	// BaseBranch is the dependency-tree root, used to validate the combined parent
	// graph stays acyclic once every listed worktree is reparented.
	BaseBranch string
}

// ReparentResult is the outcome of a single reparent: the branch whose parent
// changed, plus the parent before and after.
type ReparentResult struct {
	Branch    string `json:"branch"`
	OldParent string `json:"old_parent"`
	NewParent string `json:"new_parent"`
}

// CleanReparentPlan lists the reparenting proposed when cleaning a worktree that
// is the parent of others: each child would move from the cleaned branch to the
// grandparent. It is computed before deletion so the command can show a recap.
type CleanReparentPlan struct {
	// Branch is the worktree about to be cleaned.
	Branch string
	// Grandparent is the parent the children would be reparented onto.
	Grandparent string
	Children    []ReparentResult
}

// CleanResult is the outcome of a clean, including any children reparented onto
// the grandparent.
type CleanResult struct {
	Branch        string           `json:"branch"`
	Path          string           `json:"path"`
	AlreadyAbsent bool             `json:"already_absent"`
	Reparented    []ReparentResult `json:"reparented,omitempty"`
}

// CleanCheckResult holds the pre-deletion check results.
type CleanCheckResult struct {
	WorktreePath    string
	Branch          string
	UnpushedCommits int
	HasOpenPR       bool
	PRUrl           string
	IsDirty         bool
	IsParent        bool
}

// ListParams holds inputs for listing worktrees with status.
type ListParams struct {
	ProjectDir string
	StateDir   string
	Config     Config
}

// ResolveParams holds inputs for resolving a branch query to a worktree path.
type ResolveParams struct {
	ProjectDir string
	Query      string
}

// ResolveResult indicates whether the resolution is direct or needs a picker.
type ResolveResult struct {
	Path string
	// Branch is the worktree branch backing Path. Set on a direct (non-ambiguous)
	// resolution so `resolve --output json` can report {path, branch}.
	Branch    string
	Ambiguous bool
	Matches   []GitWorktree
}

// CleanBlocker names one reason a removal is refused. Listing them one by one is
// what lets a surface have each lifted on its own, instead of behind a single
// blanket "force".
type CleanBlocker struct {
	Key   string
	Label string
}
