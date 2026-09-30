package detect

import (
	"os"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
)

type NextConfigParams struct {
	// WorkDir is the worktree root; Cwd is the job's own directory relative to
	// it, which is where a monorepo's Next app actually lives.
	WorkDir string
	Cwd     string
}

// NextConfig is a job's next.config.*, empty when it has none. Both the path and
// the source come back: the warning has to name the file the user must edit.
func NextConfig(params NextConfigParams) (path, source string) {
	dir := params.WorkDir
	if params.Cwd != "" {
		dir = filepath.Join(params.WorkDir, params.Cwd)
	}

	for _, name := range domain.DevOriginsFiles {
		candidate := filepath.Join(dir, name)
		content, err := os.ReadFile(candidate)
		if err != nil {
			continue
		}
		return candidate, string(content)
	}
	return "", ""
}
