package rules

import (
	"slices"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

type JobStateOfParams struct {
	Status   domain.JobStatus
	Detached bool
}

// JobStateOf reads a daemon status as the lifecycle state a reader shares. A
// claim has none: it is a worktree's hold on a service, reported where that
// service runs. A reaped job is stopped, not crashed: wtm took it down.
func JobStateOf(params JobStateOfParams) (domain.JobState, bool) {
	switch params.Status {
	case domain.JobStatusRunning:
		if params.Detached {
			return domain.JobStateStarting, true
		}
		return domain.JobStateRunning, true
	case domain.JobStatusDetached:
		return domain.JobStateRunning, true
	case domain.JobStatusCrashed:
		return domain.JobStateCrashed, true
	case domain.JobStatusStopped, domain.JobStatusReaped:
		return domain.JobStateStopped, true
	default:
		return "", false
	}
}

type LastLinesParams struct {
	Text  string
	Count int
}

func LastLines(params LastLinesParams) []string {
	if params.Text == "" || params.Count <= 0 {
		return nil
	}
	lines := strings.Split(params.Text, "\n")
	return lines[max(0, len(lines)-params.Count):]
}

type WorktreeJobsParams struct {
	Path string
	Jobs []domain.JobInfo
}

// WorktreeJobs is what a snapshot reports of one worktree's jobs, by name. A
// daemon too old to report a state has it read from its status.
func WorktreeJobs(params WorktreeJobsParams) []domain.JobSnapshot {
	jobs := []domain.JobSnapshot{}
	for _, info := range params.Jobs {
		if info.WorkDir != params.Path {
			continue
		}
		state, reported := stateOfInfo(info)
		if !reported {
			continue
		}
		jobs = append(jobs, domain.JobSnapshot{Name: info.Name, Kind: info.Kind, State: state, URL: info.URL, ExitCode: crashCode(crashCodeParams{State: state, Code: info.ExitCode})})
	}
	slices.SortFunc(jobs, func(a, b domain.JobSnapshot) int { return strings.Compare(a.Name, b.Name) })
	return jobs
}

type crashCodeParams struct {
	State domain.JobState
	Code  *int
}

// crashCode drops the -1 a stop's signal leaves on a stopped job: only a
// crash's code says something about the job.
func crashCode(params crashCodeParams) *int {
	if params.State != domain.JobStateCrashed {
		return nil
	}
	return params.Code
}

func stateOfInfo(info domain.JobInfo) (domain.JobState, bool) {
	if info.State != "" {
		return info.State, true
	}
	return JobStateOf(JobStateOfParams{Status: info.Status})
}
