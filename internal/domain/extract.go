package domain

// ExtractFileStatus classifies an uncommitted file selected for extraction.
type ExtractFileStatus string

const (
	// ExtractStatusModified marks a tracked file with content changes vs HEAD.
	ExtractStatusModified ExtractFileStatus = "modified"

	// ExtractStatusUntracked marks a file unknown to git (porcelain status "??").
	ExtractStatusUntracked ExtractFileStatus = "untracked"

	// ExtractStatusDeleted marks a tracked file removed from the working tree.
	ExtractStatusDeleted ExtractFileStatus = "deleted"

	// ExtractStatusRenamed marks a staged rename or copy. Both the new and the
	// original path take part in the extraction patch.
	ExtractStatusRenamed ExtractFileStatus = "renamed"
)

// ExtractFile is one uncommitted file eligible for extraction to another worktree.
type ExtractFile struct {
	Path   string            `json:"path"`
	Status ExtractFileStatus `json:"status"`

	// OrigPath is the path a renamed or copied file came from, empty otherwise.
	OrigPath string `json:"orig_path,omitempty"`
}

// ExtractParams holds inputs for moving uncommitted files from the source
// worktree to the target worktree.
type ExtractParams struct {
	SourcePath   string
	SourceBranch string
	TargetPath   string
	TargetBranch string
	Files        []ExtractFile
	Keep         bool
	// ConflictMode selects what happens when the changes do not apply cleanly:
	// OnConflictAbort (default) leaves everything untouched; OnConflictResolve
	// writes conflict markers into the target and keeps the source intact.
	ConflictMode string
}

// ConflictCheckParams holds inputs for detecting which selected files would not
// apply cleanly onto the target worktree.
type ConflictCheckParams struct {
	SourcePath string
	TargetPath string
	Files      []ExtractFile
}

// ExtractResult is the outcome of a successful extraction.
type ExtractResult struct {
	Files        []ExtractFile `json:"files"`
	TargetPath   string        `json:"target_path"`
	TargetBranch string        `json:"target_branch"`
	SourceBranch string        `json:"source_branch"`
	Kept         bool          `json:"kept"`
	// Conflicts lists the files written with conflict markers in resolve mode.
	// Empty on a clean extraction.
	Conflicts []string `json:"conflicts"`
	// Isolation is the target's; EnvPorts is the port pass of a target this
	// extraction created, as CreateResult carries it.
	Isolation Isolation   `json:"isolation,omitempty"`
	EnvPorts  EnvPortPlan `json:"env_ports,omitzero"`
	// Warnings are what the run module could not do for a target this
	// extraction created, which never fails it.
	Warnings []string `json:"warnings,omitempty"`
	// Origins is CreateResult's, for a target this extraction created.
	Origins map[string]AnswerOrigin `json:"origins,omitempty"`
}
