package wt

import (
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
)

type displayPathParams struct {
	Config     domain.Config
	ProjectDir string
	Path       string
}

// createDisplayPath renders the new worktree as base_path/<name>. One that already
// existed may sit elsewhere (adopted, or holding the branch outside base_path), and
// recomposing it there would name a directory that does not exist.
func createDisplayPath(params displayPathParams) string {
	base := params.Config.Project.Worktrees.BasePath
	expected := filepath.Join(params.ProjectDir, base, filepath.Base(params.Path))
	if filepath.Clean(expected) != filepath.Clean(params.Path) {
		return params.Path
	}
	return filepath.Join(base, filepath.Base(params.Path))
}
