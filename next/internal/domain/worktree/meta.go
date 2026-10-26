package worktree

import (
	"path/filepath"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

const metaDirName = "worktrees"

// Meta is a worktree's meta.json record, shared with wtm: what it was created from, when, with which env strategy, its number, its namespaces and its isolation.
type Meta struct {
	Schema       int                 `json:"schema,omitempty"`
	SourceBranch string              `json:"source_branch"`
	CreatedAt    string              `json:"created_at"`
	EnvStrategy  project.EnvStrategy `json:"env_strategy"`
	Ordinal      int                 `json:"ordinal,omitempty"`
	Namespaces   []string            `json:"namespaces,omitempty"`
	Isolation    project.Isolation   `json:"isolation,omitempty"`
}

// Created returns the time the record's created_at says, nil when it has none or it does not parse.
func (m Meta) Created() *time.Time {
	at, err := time.Parse(time.RFC3339, m.CreatedAt)
	if err != nil {
		return nil
	}

	return &at
}

// MetaDirParams names the branch whose record MetaDir locates, in the project's state directory.
type MetaDirParams struct {
	StateDir string
	Branch   string
}

// MetaDir returns the directory holding the branch's meta.json, and reports whether the branch has one: false for an empty state directory or a branch whose segment would leave the worktrees directory.
func MetaDir(params MetaDirParams) (string, bool) {
	segment := StateSegment(params.Branch)
	if params.StateDir == "" || segment == "" || segment == "." || segment == ".." {
		return "", false
	}

	return filepath.Join(params.StateDir, metaDirName, segment), true
}

// MetaUpdate asks to change the record of Branch with Change, which edits it in place or returns an error to leave it as it was.
type MetaUpdate struct {
	Branch string
	Change func(*Meta) error
}

// Identity is a worktree as the events payload names it, shared with wtm's listeners.
type Identity struct {
	Branch    string            `json:"branch"`
	Path      string            `json:"path"`
	Parent    string            `json:"parent"`
	Ordinal   *int              `json:"ordinal"`
	Isolation project.Isolation `json:"isolation"`
	Main      bool              `json:"is_main"`
	CreatedAt string            `json:"created_at"`
}

// IdentityParams are a worktree's branch, its path, and its record, which its identity reads.
type IdentityParams struct {
	Branch string
	Path   string
	Record Meta
}

// IdentityOf is the worktree of params as its events name it, from its record: no ordinal when the record has none.
func IdentityOf(params IdentityParams) Identity {
	record := params.Record
	identity := Identity{Branch: params.Branch, Path: params.Path, Parent: record.SourceBranch, Isolation: record.Isolation.Effective(), CreatedAt: record.CreatedAt}
	if record.Ordinal > 0 {
		identity.Ordinal = &record.Ordinal
	}

	return identity
}

// A PolledField names what the event stream polls of a worktree and found changed.
type PolledField string

// The fields of a worktree the event stream polls: its branch's tip, what it has checked out (a branch, a rebase, a bisect), and its jobs and their states.
const (
	PolledHead     PolledField = "head"
	PolledCheckout PolledField = "checkout"
	PolledJobs     PolledField = "jobs"
)

// An IdentityField names a field of an [Identity] an update changed, as the events spell it.
type IdentityField string

// The fields of an identity an update may change.
const (
	IdentityIsolation IdentityField = "isolation"
	IdentityOrdinal   IdentityField = "ordinal"
	IdentityParent    IdentityField = "parent"
	IdentityCreatedAt IdentityField = "created_at"
)
