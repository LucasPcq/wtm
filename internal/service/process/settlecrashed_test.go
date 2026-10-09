package process

import (
	"slices"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func crashedJob(t *testing.T, m *Manager, job domain.JobConfig, dir string) {
	t.Helper()
	if err := m.Start(StartParams{Job: job, WorkDir: dir}); err != nil {
		t.Fatalf("start: %v", err)
	}
	waitForJob(t, m, job.Name, func(j ManagedJob) bool { return j.Status == domain.JobStatusCrashed })
}

// LUC-276 (a): `run down` left a crashed job crashed, so `wtm status` kept
// reporting a crash the user had already put down. It settles to stopped, and
// the job.crashed already sent stays its only event.
func TestRunDownSettlesACrashedJob(t *testing.T) {
	stops := map[string]func(m *Manager, dir string) error{
		"stop all in the worktree": func(m *Manager, dir string) error { return m.StopAllInWorkDir(dir) },
		"stop all":                 func(m *Manager, _ string) error { return m.StopAll() },
		"stop the job":             func(m *Manager, dir string) error { return m.Stop(JobRef{Name: "server", WorkDir: dir}) },
	}
	for name, stop := range stops {
		t.Run(name, func(t *testing.T) {
			m, log := newObservedManager()
			dir := t.TempDir()
			crashedJob(t, m, domain.JobConfig{Name: "server", Kind: domain.JobKindService, Cmd: "exit 3"}, dir)
			log.await(t, 2)

			if err := stop(m, dir); err != nil {
				t.Fatalf("stop: %v", err)
			}
			if status := m.List()[0].Status; status != domain.JobStatusStopped {
				t.Errorf("status = %s, want stopped", status)
			}
			if slices.Contains(log.types(), domain.EventJobStopped) {
				t.Errorf("events = %v, want no job.stopped after the crash", log.types())
			}
		})
	}
}

func TestShutdownLeavesACrashedJobAsItWas(t *testing.T) {
	m := newManager()
	dir := t.TempDir()
	crashedJob(t, m, domain.JobConfig{Name: "server", Kind: domain.JobKindService, Cmd: "exit 3"}, dir)

	if err := m.StopForeground(); err != nil {
		t.Fatalf("stop foreground: %v", err)
	}
	if status := m.List()[0].Status; status != domain.JobStatusCrashed {
		t.Errorf("status = %s, want the crash kept", status)
	}
}
