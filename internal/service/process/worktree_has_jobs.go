package process

import "github.com/LucasPcq/wtm/internal/rules"

// WorktreeHasJobs says whether anything the daemon holds is keyed on workDir,
// claims on shared services included. A daemon that cannot be listed answers
// yes: a caller refusing to move the worktree is the safe side of not knowing.
func WorktreeHasJobs(workDir string) bool {
	socket := SocketPath()
	if !IsDaemonRunning(socket) {
		return HasIndexedJobs(workDir)
	}
	resp, err := NewClient(socket).Send(Request{Action: ActionList})
	if err != nil || resp.Status == StatusError {
		return true
	}
	for _, info := range resp.Jobs {
		if info.WorkDir == workDir && rules.IsJobUp(info.Status) {
			return true
		}
	}
	return false
}
