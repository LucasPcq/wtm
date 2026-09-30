package rules

import "path/filepath"

// JobDirParams names its two strings rather than ordering them: swapping a
// worktree root and a relative cwd silently resolves to a directory that exists
// and is not the job's.
type JobDirParams struct {
	WorkDir string
	Cwd     string
}

// JobDir is where a job's command runs — the one answer both the spawn and every
// later question about that job have to agree on. A compose probe resolving it
// differently asks about a project the launcher never started.
func JobDir(params JobDirParams) string {
	if params.Cwd == "" {
		return params.WorkDir
	}
	if filepath.IsAbs(params.Cwd) {
		return params.Cwd
	}
	return filepath.Join(params.WorkDir, params.Cwd)
}
