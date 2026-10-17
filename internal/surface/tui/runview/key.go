package runview

import (
	"strings"

	"github.com/LucasPcq/wtm/internal/flow/runlogs"
)

// jobKey identifies a job in the view. It is the worktree and the name
// together, never the name alone: two worktrees running the same profile hold
// two jobs called `web`, and selecting one by name would select both — the same
// pane, the same subscription, the same keystrokes.
type jobKey string

// keySep cannot occur in a path or a job name, so a key can be taken apart
// again without escaping either half.
const keySep = "\x00"

func jobKeyOf(workDir, job string) jobKey {
	if job == "" {
		return ""
	}
	// A job whose worktree is unknown keys on its name alone. It is the same key
	// the view used before there was a worktree axis, which is what keeps a
	// board that names no worktree behaving exactly as it did.
	if workDir == "" {
		return jobKey(job)
	}
	return jobKey(workDir + keySep + job)
}

func viewKey(view runlogs.JobView) jobKey { return jobKeyOf(view.WorkDir, view.Name) }

func eventKey(event runlogs.Event) jobKey { return jobKeyOf(event.WorkDir, event.Job) }

func (k jobKey) job() string {
	_, job, found := strings.Cut(string(k), keySep)
	if !found {
		return string(k)
	}
	return job
}

func (k jobKey) workDir() string {
	workDir, _, found := strings.Cut(string(k), keySep)
	if !found {
		return ""
	}
	return workDir
}
