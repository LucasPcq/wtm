package worktree

import (
	"path/filepath"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/wtm/project"
)

const metaDirName = "worktrees"

type Meta struct {
	Schema       int                 `json:"schema,omitempty"`
	SourceBranch string              `json:"source_branch"`
	CreatedAt    string              `json:"created_at"`
	EnvStrategy  project.EnvStrategy `json:"env_strategy"`
	Ordinal      int                 `json:"ordinal,omitempty"`
	Namespaces   []string            `json:"namespaces,omitempty"`
	Isolation    project.Isolation   `json:"isolation,omitempty"`
}

type MetaDirParams struct {
	StateDir string
	Branch   string
}

func MetaDir(params MetaDirParams) (string, bool) {
	segment := StateSegment(params.Branch)
	if params.StateDir == "" || segment == "" || segment == "." || segment == ".." || strings.ContainsAny(segment, `/\`) {
		return "", false
	}
	return filepath.Join(params.StateDir, metaDirName, segment), true
}

type MetaUpdate struct {
	Branch string
	Change func(*Meta) error
}

type Identity struct {
	Branch    string            `json:"branch"`
	Path      string            `json:"path"`
	Parent    string            `json:"parent"`
	Ordinal   *int              `json:"ordinal"`
	Isolation project.Isolation `json:"isolation"`
	Main      bool              `json:"is_main"`
	CreatedAt string            `json:"created_at"`
}
