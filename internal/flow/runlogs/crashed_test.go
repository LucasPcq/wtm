package runlogs_test

import (
	"context"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/testutil/runlogstest"
)

// The daemon accepts a spawn, not a life: a service that binds a busy port is
// accepted and gone a moment later. The run must not conclude "started" over a
// process that has already exited.
func TestARunSettlesAJobThatDiedRightAfterStarting(t *testing.T) {
	code := 1
	service := &runlogstest.Service{
		Infos: []domain.JobInfo{
			{Name: "web", WorkDir: "/wt/a", Status: domain.JobStatusCrashed, ExitCode: &code},
		},
	}

	outcome, err := runlogs.Run(context.Background(), runlogs.RunParams{
		Service: service,
		WorkDir: "/wt/a",
		Jobs:    []domain.JobConfig{{Name: "web", Kind: domain.JobKindService}},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if len(outcome.Started) != 0 {
		t.Errorf("Started = %v, want the dead job out of it", outcome.Started)
	}
	if len(outcome.Crashed) != 1 || outcome.Crashed[0].Job != "web" {
		t.Fatalf("Crashed = %+v, want web", outcome.Crashed)
	}
	if outcome.Crashed[0].ExitCode == nil || *outcome.Crashed[0].ExitCode != 1 {
		t.Errorf("exit code = %v, want 1", outcome.Crashed[0].ExitCode)
	}
	if len(outcome.Results) != 1 || outcome.Results[0].Status != domain.JobActionCrashed {
		t.Errorf("results = %+v, want the recorded result corrected", outcome.Results)
	}
}

func TestARunKeepsAJobTheDaemonStillHoldsUp(t *testing.T) {
	service := &runlogstest.Service{
		Infos: []domain.JobInfo{
			{Name: "web", WorkDir: "/wt/a", Status: domain.JobStatusRunning},
		},
	}

	outcome, err := runlogs.Run(context.Background(), runlogs.RunParams{
		Service: service,
		WorkDir: "/wt/a",
		Jobs:    []domain.JobConfig{{Name: "web", Kind: domain.JobKindService}},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if len(outcome.Started) != 1 || len(outcome.Crashed) != 0 {
		t.Errorf("Started = %v, Crashed = %+v, want the job left alone", outcome.Started, outcome.Crashed)
	}
}

// A job that died takes its port report with it: an exit code says more than a
// silent port, and two complaints about one failure read as two failures.
func TestACrashedJobsPortReportIsWithdrawn(t *testing.T) {
	code := 1
	service := &runlogstest.Service{
		Ports: map[string]map[string]int{"web": {"PORT": 3000}},
		Infos: []domain.JobInfo{
			{Name: "web", WorkDir: "/wt/a", Status: domain.JobStatusCrashed, ExitCode: &code},
		},
	}

	outcome, err := runlogs.Run(context.Background(), runlogs.RunParams{
		Service: service,
		WorkDir: "/wt/a",
		Jobs:    []domain.JobConfig{{Name: "web", Kind: domain.JobKindService, Ports: map[string]int{"PORT": 3000}}},
		Prober:  silentProber{},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if len(outcome.Probes) != 0 {
		t.Errorf("Probes = %+v, want none for a job that is gone", outcome.Probes)
	}
	if len(outcome.Crashed) != 1 {
		t.Errorf("Crashed = %+v, want the exit reported instead", outcome.Crashed)
	}
}

// silentProber answers that nothing is listening, which is what a crashed job
// leaves behind.
type silentProber struct{}

func (silentProber) Listening(context.Context, []int, func(map[int]bool) bool) map[int]bool {
	return map[int]bool{}
}
