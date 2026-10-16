package worktree

import (
	"path/filepath"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/wtm/branch"
	"github.com/LucasPcq/wtm/next/internal/wtm/project"
	"github.com/LucasPcq/wtm/next/internal/wtm/run"
)

const metaDirName = "worktrees"

type Meta struct {
	Schema       int                 `json:"schema,omitempty"`
	SourceBranch string              `json:"source_branch"`
	CreatedAt    string              `json:"created_at"`
	EnvStrategy  project.EnvStrategy `json:"env_strategy"`
	Ordinal      int                 `json:"ordinal,omitempty"`
	Namespaces   []string            `json:"namespaces,omitempty"`
	Isolation    run.Isolation       `json:"isolation,omitempty"`
}

type MetaDirParams struct {
	StateDir string
	Branch   string
}

func MetaDir(params MetaDirParams) (string, bool) {
	segment := branch.StateSegment(params.Branch)
	if params.StateDir == "" || segment == "" || segment == "." || segment == ".." || strings.ContainsAny(segment, `/\`) {
		return "", false
	}
	return filepath.Join(params.StateDir, metaDirName, segment), true
}
