package process

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func indexedOne(t *testing.T, params indexedJobsParams) domain.JobInfo {
	t.Helper()
	jobs := indexedJobs(params)
	if len(jobs) != 1 {
		t.Fatalf("jobs = %+v, want one", jobs)
	}
	return jobs[0]
}

func TestAnIndexedServiceStillAliveReadsRunningAndIsNotReaped(t *testing.T) {
	orphans := &fakeOrphans{states: aliveAndOurs()}

	job := indexedOne(t, indexedJobsParams{
		Records: []domain.JobRecord{foregroundRecord(t, t.TempDir())},
		Orphans: orphans,
		Stacks:  &unknownStacks{},
	})

	if job.Status != domain.JobStatusRunning {
		t.Errorf("status = %q, want running: its group is alive and ours", job.Status)
	}
	if len(orphans.reaped) != 0 {
		t.Errorf("reaped %v: a read never kills anything", orphans.reaped)
	}
}

func TestAnIndexedServiceWhoseGroupIsGoneReadsCrashed(t *testing.T) {
	job := indexedOne(t, indexedJobsParams{
		Records: []domain.JobRecord{foregroundRecord(t, t.TempDir())},
		Orphans: &fakeOrphans{states: map[int]GroupState{}},
		Stacks:  &unknownStacks{},
	})

	if job.Status != domain.JobStatusCrashed {
		t.Errorf("status = %q, want crashed: it died with no daemon to report it", job.Status)
	}
}

func TestAnIndexedStackReadsDetachedUntilVerifiedDown(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]struct {
		stacks Stacks
		want   domain.JobStatus
	}{
		"unknown":  {stacks: &unknownStacks{}, want: domain.JobStatusDetached},
		"verified": {stacks: &verifiedStacks{up: false}, want: domain.JobStatusStopped},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			job := indexedOne(t, indexedJobsParams{
				Records: []domain.JobRecord{detachedRecord(t, dir)},
				Orphans: &fakeOrphans{},
				Stacks:  tc.stacks,
			})
			if job.Status != tc.want {
				t.Errorf("status = %q, want %q", job.Status, tc.want)
			}
		})
	}
}

func TestAnIndexedClaimKeepsWhereItsServiceRuns(t *testing.T) {
	record := detachedRecord(t, t.TempDir())
	record.Joined = true
	record.SharedDir = "/repo"

	job := indexedOne(t, indexedJobsParams{Records: []domain.JobRecord{record}, Orphans: &fakeOrphans{}, Stacks: &unknownStacks{}})

	if job.Status != domain.JobStatusJoined || job.SharedDir != "/repo" {
		t.Errorf("job = %+v, want a claim on /repo", job)
	}
}
