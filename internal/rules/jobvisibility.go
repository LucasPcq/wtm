package rules

import (
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

// JobState is what a surface says about one job of one worktree. It is not
// domain.JobStatus: that is the daemon's word about a process it holds, and
// three of the four states here are about a job the daemon holds nothing for.
type JobState string

const (
	// JobStateUp is a job the daemon is holding right now.
	JobStateUp JobState = "up"
	// JobStateStopped is a service that ran here and was stopped.
	JobStateStopped JobState = "stopped"
	// JobStateFailed is a job that crashed, with the exit code it left.
	JobStateFailed JobState = "failed"
	// JobStateRan is a job the daemon no longer indexes whose log is on disk.
	// A task always lands here once it exits — the daemon drops it the moment it
	// does, keeping neither its exit code nor its end time — so this says it ran
	// and stops there rather than inventing a verdict.
	JobStateRan JobState = "ran"
)

type VisibleJobsParams struct {
	// Jobs is everything run.toml declares, in its own order, which is the order
	// a surface lists what it keeps.
	Jobs []domain.JobConfig
	// Up is what the daemon holds in this worktree, by job name.
	Up map[string]domain.JobInfo
	// Traces names the jobs that left output in this worktree's log directory.
	// Only a surface whose subject is the archive passes it — the logs view. A
	// surface about the present passes none: the log directory holds the current
	// run (`run up` clears what it is not starting), but a job of that run which
	// has already finished is history, and history on a state panel is what made
	// it unreadable.
	Traces map[string]bool
}

// VisibleJob is one job a surface shows and the state it shows it in.
type VisibleJob struct {
	Job   domain.JobConfig
	State JobState
	Info  domain.JobInfo
}

// VisibleJobs keeps the jobs a surface has something true to say about:
//
//	shown = the daemon indexes it  OR  (Traces given AND it left one)
//
// The index is this session — up, stopped, crashed — and it is all a surface
// about the present reads. Traces are the archive, passed only by the logs view.
//
// Hidden is what run.toml declares beyond that, which a surface offers as a
// catalogue rather than laying flat among the jobs that are actually about this
// worktree. On a monorepo declaring fifteen jobs, three of which are up, the
// old rule drew twelve rows saying "down" — of which four were tasks, which
// never run at all — and the reader had to find the three that mattered among
// them.
//
// A job held by a running runner is not hidden here: whether its address is
// folded under its runner is the panel's business, and a rule that dropped it
// would lose the only place its log can be reached from.
func VisibleJobs(params VisibleJobsParams) (visible []VisibleJob, hidden int) {
	visible = make([]VisibleJob, 0, len(params.Jobs))
	for _, job := range params.Jobs {
		info, indexed := params.Up[job.Name]
		state, shown := jobStateOf(jobStateParams{Info: info, Indexed: indexed, Traced: params.Traces[job.Name]})
		if !shown {
			hidden++
			continue
		}
		visible = append(visible, VisibleJob{Job: job, State: state, Info: info})
	}
	return visible, hidden
}

type jobStateParams struct {
	Info    domain.JobInfo
	Indexed bool
	Traced  bool
}

// jobStateOf reads the index first and the disk second. The index is the live
// answer, and a job it still holds is never described by a log that outlived an
// earlier run of it.
func jobStateOf(params jobStateParams) (state JobState, shown bool) {
	if params.Indexed {
		switch {
		case IsJobUp(params.Info.Status):
			return JobStateUp, true
		case params.Info.Status == domain.JobStatusCrashed, params.Info.Status == domain.JobStatusReaped:
			return JobStateFailed, true
		default:
			return JobStateStopped, true
		}
	}
	if params.Traced {
		return JobStateRan, true
	}
	return "", false
}

// JobStateGlyph is the mark a row wears. It is the whole vocabulary in one
// place: a surface that spelled its own would be free to call a task "down".
func JobStateGlyph(state JobState) string {
	switch state {
	case JobStateUp:
		return domain.DetailJobUpGlyph
	case JobStateFailed:
		return domain.DetailJobFailedGlyph
	case JobStateRan:
		return domain.DetailJobRanGlyph
	default:
		return domain.DetailJobDownGlyph
	}
}

// IndexedJobsByName is every job the daemon holds for this worktree, running or
// not. A stopped and a crashed job are both still indexed, and both have
// something to say; the index does not outlive the daemon, which is what keeps
// it to this session.
func IndexedJobsByName(infos []domain.JobInfo, workDir string) map[string]domain.JobInfo {
	indexed := make(map[string]domain.JobInfo, len(infos))
	for _, info := range infos {
		if info.WorkDir != workDir {
			continue
		}
		indexed[info.Name] = info
	}
	return indexed
}

// CountUp is what heads a section: how many of the shown jobs are actually up,
// which is not their number now that a stopped job keeps its row.
func CountUp(visible []VisibleJob) int {
	up := 0
	for _, job := range visible {
		if job.State == JobStateUp {
			up++
		}
	}
	return up
}

// VisibleJobUptime is the uptime a row carries: only a job that is up has one.
// An age on a stopped job would date a run that is over.
func VisibleJobUptime(job VisibleJob, now time.Time) string {
	if job.State != JobStateUp {
		return ""
	}
	return JobUptime(JobUptimeParams{Job: job.Info, Now: now})
}
