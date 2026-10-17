package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestCountDaemonJobsLeavesClaimsAndEndedJobsOut(t *testing.T) {
	supervised, detached := CountDaemonJobs([]domain.JobInfo{
		{Name: "api", Status: domain.JobStatusRunning},
		{Name: "db", Status: domain.JobStatusDetached},
		{Name: "pg", Status: domain.JobStatusJoined},
		{Name: "web", Status: domain.JobStatusCrashed},
		{Name: "migrate", Status: domain.JobStatusStopped},
	})
	if supervised != 1 || detached != 1 {
		t.Errorf("got %d supervised, %d detached; want 1 and 1", supervised, detached)
	}
}
