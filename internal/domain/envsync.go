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
	// IsolationAdoption is set only for a worktree that had never chosen its
	// isolation: adopted by this run, or left on its source's values.
	IsolationAdoption IsolationAdoption `json:"isolation_adoption,omitempty"`
	// Warnings name a port pass left undone, and why: the reconciliation of
	// the keys never depends on run.toml.
	Warnings []string `json:"warnings,omitempty"`
	// IsolationChanged says this run recorded a new isolation, which it only
	// does once the .env is in line with it.
	IsolationChanged bool `json:"isolation_changed,omitempty"`
	// Restored are the values wtm owns that a switch to verbatim put back to
	// the source's.
	Restored []EnvRestoredEntry `json:"restored,omitempty"`
}

// EnvRestoredEntry is one value wtm owns put back to its source's. Removed is
// a key the source does not have, so its line was dropped.
type EnvRestoredEntry struct {
	File    string `json:"file"`
	Key     string `json:"key"`
	From    string `json:"from"`
	To      string `json:"to,omitempty"`
	Removed bool   `json:"removed,omitempty"`
}
