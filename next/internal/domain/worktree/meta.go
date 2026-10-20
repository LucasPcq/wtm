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
