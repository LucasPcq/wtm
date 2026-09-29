package process

import "github.com/LucasPcq/wtm/internal/rules"

// WorktreeHasJobs tells whether stopping workDir's jobs has anything to do,
// without starting a daemon to find out: a detached stack outlives the daemon
// that launched it, so when none is listening the answer lives in the index.
func WorktreeHasJobs(workDir string) bool {
	socket := SocketPath()
	if !IsDaemonRunning(socket) {
		return HasIndexedJobs(workDir)
	}
	return hasJobsUp(NewClient(socket), workDir)
}

// StopWorktreeJobs stops every job attached to workDir via the daemon, starting
// one if none is listening. Returns true if a stop request was sent.
func StopWorktreeJobs(workDir string) bool {
	socket := SocketPath()
	if err := EnsureDaemon(DaemonParams{SocketPath: socket}); err != nil {
		return false
	}
	client := NewClient(socket)
	if !hasJobsUp(client, workDir) {
		return false
	}
	_, _ = client.Send(Request{Action: ActionStopAll, WorkDir: workDir})
	return true
}

func hasJobsUp(client *Client, workDir string) bool {
	resp, err := client.Send(Request{Action: ActionList})
	if err != nil {
		return false
	}
	for _, svc := range resp.Jobs {
		if svc.WorkDir == workDir && rules.IsJobUp(svc.Status) {
			return true
		}
	}
	return false
}
