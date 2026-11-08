package repository

import "github.com/LucasPcq/wtm/next/internal/domain/run"

// LastLinesParams name the logs to read: those of the jobs called Names in the worktree of Branch, under wtm's state StateDir, and their reader.
type LastLinesParams struct {
	StateDir string
	Branch   string
	Names    []string
	Read     func(file run.LogFile, n int) ([]run.Line, error)
}

// LastLines is the last line each job of params.Names left in its log, none for one whose log holds none or does not read: the jobs run.StandingIn finds nothing of, whose log tells whether they ran; none at all for a worktree on no branch.
func LastLines(params LastLinesParams) map[string]*run.Line {
	lines := map[string]*run.Line{}
	dir := run.LogDir(params.StateDir, params.Branch)
	if dir == "" {
		return lines
	}
	for _, name := range params.Names {
		read, _ := params.Read(run.LogFile{Dir: dir, Job: name}, 1)
		if len(read) > 0 {
			lines[name] = &read[len(read)-1]
		}
	}
	return lines
}
