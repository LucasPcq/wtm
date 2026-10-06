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
	// Branches names each worktree by its path: a claim says where its service
	// runs by path only.
	Branches map[string]string
}

// WorktreeJobs is what a snapshot reports of one worktree's jobs, by name. A
// daemon too old to report a state has it read from its status. A shared
// service the worktree holds reads as the instance it holds, and is left out
// when that instance is not found.
func WorktreeJobs(params WorktreeJobsParams) []domain.JobSnapshot {
	jobs := []domain.JobSnapshot{}
	for _, info := range params.Jobs {
		if info.WorkDir != params.Path {
			continue
		}
		job, reported := snapshotOf(snapshotOfParams{Info: info, WorktreeJobsParams: params})
		if !reported {
			continue
		}
		jobs = append(jobs, job)
	}
	slices.SortFunc(jobs, func(a, b domain.JobSnapshot) int { return strings.Compare(a.Name, b.Name) })
	return jobs
}

type snapshotOfParams struct {
	WorktreeJobsParams
	Info domain.JobInfo
}

func snapshotOf(params snapshotOfParams) (domain.JobSnapshot, bool) {
	info := params.Info
	if isClaim(info.Status) {
		return claimSnapshot(params)
	}
	state, reported := stateOfInfo(info)
	return domain.JobSnapshot{
		Name:     info.Name,
		Kind:     info.Kind,
		State:    state,
		URL:      info.URL,
		ExitCode: crashCode(crashCodeParams{State: state, Code: info.ExitCode}),
		Shared:   info.SharedDir != "",
	}, reported
}

func claimSnapshot(params snapshotOfParams) (domain.JobSnapshot, bool) {
	claim := params.Info
	if claim.SharedDir == "" {
		return domain.JobSnapshot{}, false
	}
	index := slices.IndexFunc(params.Jobs, func(info domain.JobInfo) bool {
		return info.Name == claim.Name && info.WorkDir == claim.SharedDir && !isClaim(info.Status)
	})
	if index < 0 {
		return domain.JobSnapshot{}, false
	}
	instance := params.Jobs[index]
	state, reported := stateOfInfo(instance)
	url := claim.URL
	if url == "" {
		url = instance.URL
	}
	return domain.JobSnapshot{
		Name:     claim.Name,
		Kind:     instance.Kind,
		State:    state,
		URL:      url,
		ExitCode: crashCode(crashCodeParams{State: state, Code: instance.ExitCode}),
		Shared:   true,
		Owner:    &domain.WorktreeRef{Branch: params.Branches[instance.WorkDir], Path: instance.WorkDir},
	}, reported
}

func isClaim(status domain.JobStatus) bool {
	return status == domain.JobStatusJoined || status == domain.JobStatusLegacyAttached
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
