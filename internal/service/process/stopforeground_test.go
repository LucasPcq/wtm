package process

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

// The daemon gives its whole stop DaemonStopTimeout. Stopped one after the
// other, N jobs deaf to SIGTERM took N grace periods, and the fourth or fifth
// was killed mid-cleanup by the process exiting.
func TestStopForegroundGivesEveryJobItsGraceAtOnce(t *testing.T) {
	previous := jobStopGrace
	jobStopGrace = 300 * time.Millisecond
	t.Cleanup(func() { jobStopGrace = previous })

	dir := t.TempDir()
	statePath := filepath.Join(t.TempDir(), domain.DaemonStateFileName)
	m := NewManagerWith(ManagerParams{Index: NewStateStore(statePath)})
	const jobs = 4
	for i := range jobs {
		job := domain.JobConfig{Name: fmt.Sprintf("deaf%d", i), Kind: domain.JobKindService, Cmd: "trap '' TERM; while :; do sleep 0.05; done"}
		if err := m.Start(StartParams{Job: job, WorkDir: dir}); err != nil {
			t.Fatalf("start %s: %v", job.Name, err)
		}
	}
	t.Cleanup(func() { _ = m.StopAll() })

	began := time.Now()
	if err := m.StopForeground(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if took := time.Since(began); took >= 2*jobStopGrace {
		t.Errorf("stopping %d jobs took %v, want about one grace period (%v)", jobs, took, jobStopGrace)
	}

	for _, job := range m.List() {
		if job.Status == domain.JobStatusRunning {
			t.Errorf("%s still running", job.Name)
		}
	}
	if records := NewStateStore(statePath).Load(); len(records) != 0 {
		t.Errorf("index = %+v, want nothing left up", records)
	}
}
