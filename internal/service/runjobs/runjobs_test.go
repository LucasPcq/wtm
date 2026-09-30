package runjobs

import (
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestLiveJobsForgetAStoppedJobOfARemovedWorktree(t *testing.T) {
	here := t.TempDir()
	gone := filepath.Join(t.TempDir(), "cleaned")

	got := liveJobs([]domain.JobInfo{
		{Name: "api", WorkDir: gone, Status: domain.JobStatusStopped},
		{Name: "api", WorkDir: here, Status: domain.JobStatusStopped},
	})

	if len(got) != 1 || got[0].WorkDir != here {
		t.Errorf("jobs = %+v, want only the one whose worktree is still there", got)
	}
}
