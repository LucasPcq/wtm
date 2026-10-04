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
	// Namespaces names the shared services this worktree has carved a namespace out
	// of. It is the only durable record that one exists: a claim on a shared
	// service goes with a `run stop`, and without this a clean would either give
	// back a namespace that was never created or leak one that was. It lives here
	// because the file is removed with the worktree it describes.
	Namespaces []string `json:"namespaces,omitempty"`
	// Isolation is the choice made when the worktree was created. Empty is a
	// worktree that predates the choice, and reads as IsolationIsolated — what
	// every worktree got until then.
	Isolation Isolation `json:"isolation,omitempty"`
}

// WorktreeNameClash names the live worktree whose derived name a branch would
// share, and that name.
type WorktreeNameClash struct {
	Branch string
	Name   string
}

// WorktreeStatus holds the display state of a worktree for wtm ls.
type WorktreeStatus struct {
	Branch       string
	Path         string
	IsParent     bool
	IsDirty      bool
	IsLocked     bool
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
	IsLocked         bool                `json:"is_locked"`
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
	// Isolation is recorded before any hook runs: a hook reads the worktree's
	// ports, and they depend on it.
	Isolation Isolation
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
	// OnHook receives each hook starting and finishing. A surface that reports
	// the beats itself sets it; nil leaves the runner to write them to Output.
	OnHook func(HookBeat)
}

// HookBeat is one beat of a lifecycle-hook phase: the same hook is reported
// starting, then finished. It carries facts and no rendering — whether a
// finished hook reads as a line, a glyph or nothing at all is the surface's.
type HookBeat struct {
	Cmd string
	Cwd string
	// Started distinguishes the two beats; a finished hook carries the rest.
	Started  bool
	Duration time.Duration
	// Err is empty on success, and Stderr what a failing hook wrote there.
	Err    string
	Stderr string
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
	// Isolation is the worktree's, recorded or, for one that predates the
	// choice, what it reads as.
	Isolation Isolation `json:"isolation,omitempty"`
	// EnvPorts is the port pass this run settled the fresh .env with — the
	// shape `wtm env` reports as ports. Absent when there was nothing to settle
	// or the pass could not run, and then Warnings says why.
	EnvPorts EnvPortPlan `json:"env_ports,omitzero"`
	// Warnings are what the run module could not do for the worktree, which
	// never fails its creation (a port pass left undone, and why).
	Warnings []string `json:"warnings,omitempty"`
}

// Path is set when the worktree exists but its hooks failed.
type BatchFailure struct {
	Branch   string `json:"branch"`
	Path     string `json:"path,omitempty"`
	Error    string `json:"error"`
	ExitCode int    `json:"exit_code"`
	// Privileged is a removal git refused on files only sudo can delete, which a
	// surface that cannot hand over its terminal has to name the way out of.
	Privileged bool `json:"-"`
}

type CreateBatchResult struct {
	Results []CreateResult `json:"results"`
	Failed  []BatchFailure `json:"failed"`
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
	// OnHook receives each hook starting and finishing. A surface that reports
	// the beats itself sets it; nil leaves the runner to write them to Output.
	OnHook func(HookBeat)
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

type CleanResult struct {
	Branch        string `json:"branch"`
	Path          string `json:"path"`
	AlreadyAbsent bool   `json:"already_absent"`
}

// CleanBatchResult is the clean payload, an envelope even for one worktree.
// Skipped reuses prune's reasons: dirty, unpushed, open_pr.
type CleanBatchResult struct {
	Results          []CleanResult      `json:"results"`
	Failed           []BatchFailure     `json:"failed"`
	Skipped          []PruneSkip        `json:"skipped"`
	Reparented       []ReparentResult   `json:"reparented"`
	OrphanedChildren []ReparentResult   `json:"orphaned_children"`
	Namespaces       []NamespaceOutcome `json:"namespaces"`
}

type CleanCheckEntry struct {
	Check CleanCheckResult
	Err   error
}

// CleanCheckResult holds the pre-deletion check results.
type CleanCheckResult struct {
	WorktreePath    string
	Branch          string
	UnpushedCommits int
	HasOpenPR       bool
	PRUrl           string
	IsDirty         bool
	IsLocked        bool
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

// TallyPart is one count of a result summary. A zero count is dropped: a
// conclusion counts what happened, never what did not.
type TallyPart struct {
	Count int
	Label string
}
