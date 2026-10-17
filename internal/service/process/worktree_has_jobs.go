package process

import (
	"context"

	"github.com/LucasPcq/wtm/internal/rules"
)

// WorktreeHasJobs says whether anything the daemon holds is keyed on workDir,
// claims on shared services included. A daemon that cannot be listed answers
// yes: a caller refusing to move the worktree is the safe side of not knowing.
func WorktreeHasJobs(ctx context.Context, workDir string) bool {
	socket := SocketPath()
	if !IsDaemonRunning(socket) {
		return HasIndexedJobs(workDir)
	}
	resp, err := NewClient(socket).Send(ctx, Request{Action: ActionList})
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
