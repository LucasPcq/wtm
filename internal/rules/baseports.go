package rules

import "github.com/LucasPcq/wtm/internal/domain"

// PortHolder is a worktree the daemon has a job up in, named as a reader
// recognises it.
type PortHolder struct {
	WorkDir  string
	Worktree string
}

type BasePortOwnersParams struct {
	// SelfWorkDir is the worktree being started; its own jobs never explain a
	// port that was already taken when it started.
	SelfWorkDir string
	Jobs        []domain.JobConfig
	Running     []domain.JobInfo
	// Holders names the worktrees the running jobs belong to.
	Holders []PortHolder
}

// BasePortOwners maps a declared base port to another worktree running the job
// that declares it.
//
// It names a neighbour, never a culprit: a job whose command ignores the port
// it was given binds the base port in whichever worktree it runs, so the holder
// is not necessarily the main checkout, and wtm cannot see which socket belongs
// to whom. What it can say for certain is that the same job is up next door —
// which is enough to stop blaming the local command for a busy port.
func BasePortOwners(params BasePortOwnersParams) map[int]string {
	names := make(map[string]string, len(params.Holders))
	for _, holder := range params.Holders {
		names[holder.WorkDir] = holder.Worktree
	}

	declared := make(map[string]map[string]int, len(params.Jobs))
	for _, job := range params.Jobs {
		declared[job.Name] = job.Ports
	}

	owners := map[int]string{}
	for _, info := range params.Running {
		if info.WorkDir == params.SelfWorkDir || !IsJobUp(info.Status) {
			continue
		}
		name := names[info.WorkDir]
		if name == "" {
			continue
		}
		for _, base := range declared[info.Name] {
			owners[base] = name
		}
	}
	if len(owners) == 0 {
		return nil
	}
	return owners
}
