package domain

// EnvFileResult is the reconciliation outcome for one configured env target of a
// worktree. Diff carries the per-key verdict; Source is the display label of the
// value cascade the strategy selected; Applied reports whether the reconciled
// content was written back; ParentFallback flags a "parent" strategy that sourced
// from main because the parent had no local worktree.
type EnvFileResult struct {
	Target   string      `json:"target"`
	Strategy EnvStrategy `json:"strategy"`
	Source   string      `json:"source"`
	Diff     EnvDiff     `json:"diff"`
	Applied  bool        `json:"applied"`
	// ParentBranch is the recorded parent branch consulted by the "parent" strategy
	// (empty for other strategies). ParentFallback is true when that parent had no
	// local worktree, so values came from main instead.
	ParentBranch   string `json:"parent_branch,omitempty"`
	ParentFallback bool   `json:"parent_fallback,omitempty"`
	// Unresolvable marks a configured file that exists nowhere — no value in the
	// worktree, none in any source, and no template to scaffold from. A fresh
	// project has no source either, but it has a template; this is a config
	// entry naming a path the repository does not have, and no amount of
	// syncing will ever fill it.
	Unresolvable bool `json:"unresolvable,omitempty"`
}

// EnvSyncResult is the full outcome of `wtm env` on one worktree across all its
// configured env files. Check mirrors --check (read-only diagnostic, nothing
// written).
type EnvSyncResult struct {
	Branch string          `json:"branch"`
	Mode   EnvMode         `json:"mode"`
	Check  bool            `json:"check"`
	Files  []EnvFileResult `json:"files"`
	// Ports is the [[env_port]] rewrite that followed the reconciliation. It is
	// reported even under Check, where nothing was written.
	Ports EnvPortPlan `json:"ports,omitzero"`
	// Isolation is the worktree's, which decides whether Ports can hold anything.
	Isolation Isolation `json:"isolation,omitempty"`
}
